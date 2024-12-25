// website/server.js
"use strict";
const express = require("express");
const { Pool } = require("pg");
const { Kafka } = require("kafkajs");

const app = express();
app.use(express.static(__dirname + "/public")); // Раздача статических файлов

const config = require("../config.json");
// Допустим, мы отдельно храним часть конфигурации,
// либо читаем config.yaml через yaml-parse

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
        level: level,
        message: message
    };
    producer.send({
        topic: "service-logs",
        messages: [
            { value: JSON.stringify(event) }
        ]
    }).catch(e => console.error("Kafka send error:", e));
}

// Пример эндпоинта, который возвращает список статей
app.get("/api/articles", async (req, res) => {
    try {
        logToKafka("INFO", "GET /api/articles");
        const { rows } = await pool.query("SELECT id, title, content, source_url FROM articles ORDER BY id DESC LIMIT 50");
        res.json(rows);
    } catch (err) {
        logToKafka("ERROR", "Failed to get articles: " + err.message);
        res.status(500).json({ error: "Database error" });
    }
});

// Главная страница
app.get("/", (req, res) => {
    res.sendFile(__dirname + "/public/index.html");
});

// Запуск
const port = config.website.port || 3001;
initKafka().then(() => {
    app.listen(port, () => {
        console.log(`Website is running on http://localhost:${port}`);
    });
}).catch(err => {
    console.error("Failed to init Kafka:", err);
    process.exit(1);
});
