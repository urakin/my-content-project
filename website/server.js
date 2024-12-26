"use strict";

const express = require("express");
const { Pool } = require("pg");
const { Kafka } = require("kafkajs");
const path = require("path");
const fs = require("fs");
const yaml = require("js-yaml");

function loadConfig() {
    const filePath = path.join(__dirname, "..", "config.yaml");
    const data = fs.readFileSync(filePath, "utf8");
    return yaml.load(data);
}

const config = loadConfig();

const app = express();
app.use(express.static(path.join(__dirname, "public")));

const pool = new Pool({
    host: config.database.postgres.host,
    port: config.database.postgres.port,
    user: config.database.postgres.user,
    password: config.database.postgres.password,
    database: config.database.postgres.database
});

const kafka = new Kafka({
    clientId: "website",
    brokers: config.kafka.bootstrap_servers
});

let producer;

async function initKafka() {
    producer = kafka.producer();
    await producer.connect();
}

function logToKafka(level, message) {
    const event = {
        timestamp: new Date().toISOString(),
        service: "website",
        level,
        message
    };
    producer
        .send({
            topic: "service-logs",
            messages: [{ value: JSON.stringify(event) }]
        })
        .catch(e => console.error("Kafka send error:", e));
}

// API для получения статей
app.get("/api/articles", async (req, res) => {
    try {
        logToKafka("INFO", "GET /api/articles");
        const { rows } = await pool.query("SELECT id, title, content, source_url, created_at FROM articles ORDER BY id DESC LIMIT 50");
        res.json(rows);
    } catch (err) {
        logToKafka("ERROR", "Failed to get articles: " + err.message);
        res.status(500).json({ error: "Database error" });
    }
});

// Главная страница
app.get("/", (req, res) => {
    res.sendFile(path.join(__dirname, "public", "index.html"));
});

const port = config.website.port || 3001;

initKafka()
    .then(() => {
        app.listen(port, () => {
            console.log(`Website is running on http://localhost:${port}`);
            logToKafka("INFO", `Website started on port ${port}`);
        });
    })
    .catch(err => {
        console.error("Failed to init Kafka:", err);
        process.exit(1);
    });
