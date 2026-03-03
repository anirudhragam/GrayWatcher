FROM python:3.13-slim

WORKDIR /app

COPY requirements.txt .
COPY observer.py .

RUN pip install --no-cache-dir -r requirements.txt

CMD ["python", "observer.py"]