import time
from locust import HttpUser, task, between
import random
import csv

reader = csv.reader(open('codes.csv'))
rowlist = list(reader)
characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
length = len(characters)
def generateRandomString():
    genStr = ""
    for i in range(5):
        genStr = ''.join([genStr, characters[random.randint(0, 61)]])
    return genStr

class QuickstartUser(HttpUser):
    wait_time = between(1, 5)
    host = "http://localhost:8080"
    
    @task
    def get(self):
        # self.client.get("/short/" + rowlist[random.randint(0, 2000)][0])
        self.client.get("/short/" + 'mc02M')