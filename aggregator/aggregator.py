#!/usr/bin/env python3
# aggregator.py

import os
import json
import datetime
import requests
import psycopg2
import yaml
from kafka import KafkaProducer

def load_config():
    with open("/app/config.yaml", 'r', encoding='utf-8') as f:
        return yaml.safe_load(f)

def connect_postgres(pg_conf):
    return psycopg2.connect(
        host=pg_conf["host"],
        port=pg_conf["port"],
        dbname=pg_conf["database"],
        user=pg_conf["user"],
        password=pg_conf["password"]
    )

def init_kafka_producer(kafka_conf):
    return KafkaProducer(
        bootstrap_servers=kafka_conf["bootstrap_servers"],
        value_serializer=lambda v: json.dumps(v).encode('utf-8')
    )

def log_to_kafka(producer, service, level, message):
    event = {
        "timestamp": datetime.datetime.utcnow().isoformat(),
        "service": service,
        "level": level.upper(),
        "message": message
    }
    producer.send("service-logs", value=event)

def fetch_newsapi_articles(api_key, query):
    url = "https://newsapi.org/v2/everything"
    params = {
        "q": query,
        "language": "en",
        "pageSize": 5,
        "apiKey": api_key
    }
    try:
        resp = requests.get(url, params=params)
        resp.raise_for_status()
        data = resp.json()
        return data.get("articles", [])
    except Exception as e:
        print(f"Error fetching NewsAPI: {e}")
        return []

def insert_article(conn, title, content, source_url):
    with conn.cursor() as cur:
        cur.execute("""
            INSERT INTO articles (title, content, source_url)
            VALUES (%s, %s, %s)
            RETURNING id
        """, (title, content, source_url))
        row_id = cur.fetchone()[0]
        conn.commit()
        return row_id

def main():
    config = load_config()
    pg_conf = config["database"]["postgres"]
    kafka_conf = config["kafka"]
    aggregator_conf = config["aggregator"]

    conn = connect_postgres(pg_conf)
    producer = init_kafka_producer(kafka_conf)

    log_to_kafka(producer, "aggregator", "INFO", "Aggregator started")

    keywords = aggregator_conf.get("keywords", "technology")
    if aggregator_conf.get("useNewsAPI", False):
        api_key = config["services"]["newsapi"]["apiKey"]
        articles = fetch_newsapi_articles(api_key, keywords)
        for art in articles:
            title = art.get("title", "No title")
            description = art.get("description", "")
            url = art.get("url", "")
            article_id = insert_article(conn, title, description, url)
            log_to_kafka(producer, "aggregator", "DEBUG", f"Inserted article id={article_id}")

    # Дополнительно можно прописать fetch_reddit_rss, fetch_bing и прочие источники

    log_to_kafka(producer, "aggregator", "INFO", "Aggregator finished")

    producer.flush()
    producer.close()
    conn.close()

if __name__ == "__main__":
    main()
