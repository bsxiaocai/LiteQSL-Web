import os

DATABASE_PATH = os.path.join(os.path.dirname(__file__), "data", "qsl.db")
SECRET_KEY = os.getenv("SECRET_KEY", "change-this-secret-key-in-production")
