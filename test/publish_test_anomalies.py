#!/usr/bin/env python3
"""
Publish test anomalies to Pub/Sub emulator for integration testing.
"""

import json
import os
import time
from datetime import datetime

from google.cloud import pubsub_v1

def publish_test_anomalies(project_id="test-project", topic_id="anomaly-events-topic", count=5):
    """Publish test anomalies to Pub/Sub."""
    
    # Check if using emulator
    emulator_host = os.getenv("PUBSUB_EMULATOR_HOST")
    if emulator_host:
        print(f"Using Pub/Sub emulator at {emulator_host}")
        client_options = {
            "api_endpoint": emulator_host
        }
        publisher = pubsub_v1.PublisherClient(client_options=client_options)
    else:
        print("Using real Pub/Sub (set PUBSUB_EMULATOR_HOST for emulator)")
        publisher = pubsub_v1.PublisherClient()
    
    topic_path = publisher.topic_path(project_id, topic_id)
    
    # Create test anomalies
    anomalies = []
    for i in range(count):
        timestamp = int(time.time()) - i * 3600  # One hour apart
        anomaly = {
            "id": f"test-anomaly-{i}-{timestamp}",
            "title": f"Test Anomaly {i}",
            "severity": "critical" if i % 3 == 0 else "warning",
            "message": f"Test anomaly message {i}. This is a simulated anomaly for testing.",
            "timestamp": timestamp
        }
        anomalies.append(anomaly)
    
    # Create payload
    payload = {"anomalies": anomalies}
    data = json.dumps(payload).encode("utf-8")
    
    # Publish
    print(f"Publishing {len(anomalies)} anomalies to {topic_path}...")
    future = publisher.publish(topic_path, data=data)
    message_id = future.result(timeout=10)
    
    print(f"Published message {message_id}")
    print(f"Payload: {json.dumps(payload, indent=2)}")
    
    return message_id

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Publish test anomalies to Pub/Sub")
    parser.add_argument("--project", default="test-project", help="Google Cloud project ID")
    parser.add_argument("--topic", default="anomaly-events-topic", help="Pub/Sub topic ID")
    parser.add_argument("--count", type=int, default=3, help="Number of anomalies to publish")
    parser.add_argument("--emulator", help="Pub/Sub emulator host (e.g., localhost:8085)")
    
    args = parser.parse_args()
    
    if args.emulator:
        os.environ["PUBSUB_EMULATOR_HOST"] = args.emulator
    
    publish_test_anomalies(args.project, args.topic, args.count)