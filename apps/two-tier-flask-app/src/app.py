import os
import time
import logging
from flask import Flask, render_template, request, jsonify
from flask_mysqldb import MySQL
from pythonjsonlogger import jsonlogger

# --------------------------------------------------
# Structured JSON Logging Configuration
# --------------------------------------------------
logger = logging.getLogger()
logger.setLevel(logging.INFO)

logHandler = logging.StreamHandler()
formatter = jsonlogger.JsonFormatter(
    "%(asctime)s %(levelname)s %(message)s"
)
logHandler.setFormatter(formatter)
logger.addHandler(logHandler)

# --------------------------------------------------
# Flask App Setup
# --------------------------------------------------
app = Flask(__name__)

# MySQL configuration from environment variables
app.config["MYSQL_HOST"] = os.environ.get("MYSQL_HOST", "localhost")
app.config["MYSQL_USER"] = os.environ.get("MYSQL_USER", "default_user")
app.config["MYSQL_PASSWORD"] = os.environ.get("MYSQL_PASSWORD", "default_password")
app.config["MYSQL_DB"] = os.environ.get("MYSQL_DB", "default_db")

mysql = MySQL(app)

# --------------------------------------------------
# Database Initialization (K8s Safe Retry Logic)
# --------------------------------------------------
def init_db():
    retries = 10
    while retries > 0:
        try:
            with app.app_context():
                cur = mysql.connection.cursor()
                cur.execute("""
                    CREATE TABLE IF NOT EXISTS messages (
                        id INT AUTO_INCREMENT PRIMARY KEY,
                        message TEXT
                    );
                """)
                mysql.connection.commit()
                cur.close()

            logger.info("Database initialized successfully")
            return

        except Exception as e:
            logger.error(
                "DB not ready, retrying",
                extra={"error": str(e), "retries_left": retries}
            )
            retries -= 1
            time.sleep(5)

    logger.error("Failed to initialize DB after multiple retries")

# --------------------------------------------------
# Request Logging Middleware (Production-Style)
# --------------------------------------------------
@app.before_request
def start_timer():
    request.start_time = time.time()

@app.after_request
def log_request(response):
    duration = round((time.time() - request.start_time) * 1000, 2)

    logger.info(
        "request completed",
        extra={
            "method": request.method,
            "path": request.path,
            "status": response.status_code,
            "duration_ms": duration,
            "remote_addr": request.remote_addr,
        },
    )

    return response

# --------------------------------------------------
# Health Endpoint (Readiness Probe)
# --------------------------------------------------
@app.route("/health")
def health():
    try:
        cur = mysql.connection.cursor()
        cur.execute("SELECT 1")
        cur.close()
        return "ok", 200
    except Exception as e:
        logger.error("Health check failed", extra={"error": str(e)})
        return "db not ready", 503

# --------------------------------------------------
# Main Routes
# --------------------------------------------------
@app.route("/")
def hello():
    try:
        cur = mysql.connection.cursor()
        cur.execute("SELECT message FROM messages")
        messages = cur.fetchall()
        cur.close()

        logger.info("Fetched messages successfully")
        return render_template("index.html", messages=messages)

    except Exception as e:
        logger.error(
            "Database query failed",
            extra={"error": str(e)}
        )
        raise

@app.route("/submit", methods=["POST"])
def submit():
    try:
        new_message = request.form.get("new_message")

        cur = mysql.connection.cursor()
        cur.execute("INSERT INTO messages (message) VALUES (%s)", [new_message])
        mysql.connection.commit()
        cur.close()

        logger.info(
            "Message inserted",
            extra={"message_length": len(new_message)}
        )

        return jsonify({"message": new_message})

    except Exception as e:
        logger.error(
            "Insert failed",
            extra={"error": str(e)}
        )
        return jsonify({"error": "insert failed"}), 500

# --------------------------------------------------
# Application Entry
# --------------------------------------------------
if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000)