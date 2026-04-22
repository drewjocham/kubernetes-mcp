#!/usr/bin/env python3
import os
from google.cloud import pubsub_v1

os.environ["PUBSUB_EMULATOR_HOST"] = "localhost:8085"
project_id = "test-project"
topic_id = "anomaly-events-topic"
subscription_id = "anomaly-events-sub"

subscriber = pubsub_v1.SubscriberClient()
topic_path = subscriber.topic_path(project_id, topic_id)
subscription_path = subscriber.subscription_path(project_id, subscription_id)

try:
    subscription = subscriber.create_subscription(
        request={"name": subscription_path, "topic": topic_path}
    )
    print(f"Subscription created: {subscription.name}")
except Exception as e:
    print(f"Error creating subscription: {e}")
    # Try to list subscriptions
    print("Listing subscriptions:")
    for sub in subscriber.list_subscriptions(request={"project": f"projects/{project_id}"}):
        print(f"  {sub.name}")