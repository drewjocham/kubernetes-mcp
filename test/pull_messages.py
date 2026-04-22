#!/usr/bin/env python3
import os
from google.cloud import pubsub_v1

os.environ["PUBSUB_EMULATOR_HOST"] = "localhost:8085"
project_id = "test-project"
subscription_id = "anomaly-events-sub"

subscriber = pubsub_v1.SubscriberClient()
subscription_path = subscriber.subscription_path(project_id, subscription_id)

def callback(message):
    print(f"Received message: {message.data.decode()}")
    message.ack()

print(f"Pulling messages from {subscription_path}...")
future = subscriber.subscribe(subscription_path, callback)
try:
    future.result(timeout=10)
except Exception as e:
    print(f"Timeout or error: {e}")
    future.cancel()