import kubernetes
import requests
import time


if __name__ == "__main__":
    print("Infrastructure Observer started")

    while True:
        print("Observer is alive")
        time.sleep(30)  