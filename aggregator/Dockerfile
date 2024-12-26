# aggregator/Dockerfile
FROM python:3.10-slim

WORKDIR /app
COPY aggregator.py /app/
COPY requirements.txt /app/
COPY ../config.yaml /app/config.yaml

RUN pip install --no-cache-dir -r requirements.txt

CMD ["python", "aggregator.py"]
