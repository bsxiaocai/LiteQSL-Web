from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from fastapi.middleware.gzip import GZipMiddleware
import os
from app.auth import setup_session_middleware
from app.database import init_db
from app.routes import public, admin
from app.version import APP_VERSION


@asynccontextmanager
async def lifespan(app):
    init_db()
    yield


app = FastAPI(title="LiteQSL-Web", version=APP_VERSION, lifespan=lifespan)
app.add_middleware(GZipMiddleware, minimum_size=500)

setup_session_middleware(app)

app.include_router(public.router)
app.include_router(admin.router)

static_dir = os.path.join(os.path.dirname(os.path.dirname(__file__)), "static")
app.mount("/static", StaticFiles(directory=static_dir), name="static")


@app.get("/")
def index():
    return FileResponse(os.path.join(static_dir, "index.html"))


@app.get("/admin")
def admin_page():
    return FileResponse(os.path.join(static_dir, "admin.html"))


@app.get("/health")
def health():
    return {"status": "ok", "version": APP_VERSION}
