#!/usr/bin/env python3
import os
from google.cloud import pubsub_v1

os.environ["PUBSUB_EMULATOR_HOST"] = "localhost:8085"
project_id = "test-project"
topic_id = "anomaly-events-topic"

publisher = pubsub_v1.PublisherClient()
topic_path = publisher.topic_path(project_id, topic_id)

try:
    topic = publisher.create_topic(request={"name": topic_path})
    print(f"Topic created: {topic.name}")
except Exception as e:
    print(f"Error creating topic: {e}")
    # Try to list topics
    print("Listing topics:")
    for topic in publisher.list_topics(request={"project": f"projects/{project_id}"}):
        print(f"  {topic.name}")