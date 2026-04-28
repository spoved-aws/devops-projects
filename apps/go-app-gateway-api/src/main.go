package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/lib/pq"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// ------------------- Prometheus Metrics -------------------

var (
	addGoalCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "add_goal_requests_total",
		Help: "Total number of add goal requests",
	})

	removeGoalCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "remove_goal_requests_total",
		Help: "Total number of remove goal requests",
	})

	httpRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(addGoalCounter)
	prometheus.MustRegister(removeGoalCounter)
	prometheus.MustRegister(httpRequestsCounter)
}

// ------------------- DB Connection -------------------

func createConnection() (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"user=%s password=%s host=%s port=%s dbname=%s sslmode=%s",
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
		os.Getenv("SSL"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// ------------------- OpenTelemetry Setup -------------------

func initTracer() func() {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint("alloy.monitoring.svc.cluster.local:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	log.Println("OTEL exporter initialized")  // 👈 ADD THIS

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("go-app-gateway-api"),
		)),
	)

	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("error shutting down tracer provider: %v", err)
		}
	}
}

// ------------------- MAIN -------------------

func main() {
	shutdown := initTracer()
	defer shutdown()

	tracer := otel.Tracer("go-app")

	router := gin.Default()

	router.LoadHTMLGlob(os.Getenv("KO_DATA_PATH") + "/*")

	db, err := createConnection()
	if err != nil {
		log.Println("Error connecting to PostgreSQL", err)
		return
	}
	defer db.Close()

	// ------------------- GET / -------------------

	router.GET("/", func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "GET /")
		defer span.End()

		rows, err := db.QueryContext(ctx, "SELECT * FROM goals")
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "db query failed")

			log.Println("Error querying database", err)
			c.String(http.StatusInternalServerError, "Error querying the database")
			return
		}
		defer rows.Close()

		var goals []struct {
			ID   int
			Name string
		}

		for rows.Next() {
			var goal struct {
				ID   int
				Name string
			}

			if err := rows.Scan(&goal.ID, &goal.Name); err != nil {
				span.RecordError(err)
				log.Println("Error scanning row", err)
				continue
			}

			goals = append(goals, goal)
		}

		httpRequestsCounter.WithLabelValues("/").Inc()

		c.HTML(http.StatusOK, "index.html", gin.H{
			"goals": goals,
		})
	})

	// ------------------- POST /add_goal -------------------

	router.POST("/add_goal", func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "POST /add_goal")
		defer span.End()

		goalName := c.PostForm("goal_name")

		if goalName != "" {
			_, err := db.ExecContext(ctx, "INSERT INTO goals (goal_name) VALUES ($1)", goalName)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "insert failed")

				log.Println("Error inserting goal", err)
				c.String(http.StatusInternalServerError, "Error inserting goal")
				return
			}

			addGoalCounter.Inc()
			httpRequestsCounter.WithLabelValues("/add_goal").Inc()
		}

		c.Redirect(http.StatusFound, "/")
	})

	// ------------------- POST /remove_goal -------------------

	router.POST("/remove_goal", func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "POST /remove_goal")
		defer span.End()

		goalID := c.PostForm("goal_id")

		if goalID != "" {
			_, err := db.ExecContext(ctx, "DELETE FROM goals WHERE id = $1", goalID)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "delete failed")

				log.Println("Error deleting goal", err)
				c.String(http.StatusInternalServerError, "Error deleting goal")
				return
			}

			removeGoalCounter.Inc()
			httpRequestsCounter.WithLabelValues("/remove_goal").Inc()
		}

		c.Redirect(http.StatusFound, "/")
	})

	// ------------------- HEALTH -------------------

	router.GET("/health", func(c *gin.Context) {
		httpRequestsCounter.WithLabelValues("/health").Inc()
		c.String(http.StatusOK, "OK")
	})

	// ------------------- METRICS -------------------

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.Run(":8080")
}