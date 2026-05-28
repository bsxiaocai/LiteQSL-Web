from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
import os
from app.auth import setup_session_middleware
from app.database import init_db
from app.routes import public, admin

app = FastAPI(title="LiteQSL-Web", version="1.0.0")

setup_session_middleware(app)

app.include_router(public.router)
app.include_router(admin.router)

static_dir = os.path.join(os.path.dirname(os.path.dirname(__file__)), "static")
app.mount("/static", StaticFiles(directory=static_dir), name="static")


@app.on_event("startup")
def startup():
    init_db()


@app.get("/")
def index():
    return FileResponse(os.path.join(static_dir, "index.html"))


@app.get("/admin")
def admin_page():
    return FileResponse(os.path.join(static_dir, "admin.html"))
