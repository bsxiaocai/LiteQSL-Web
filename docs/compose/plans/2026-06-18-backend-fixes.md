# Backend Fixes Implementation Plan

> **Goal:** Fix 10 identified backend issues in LiteQSL-Web

**Files to modify:**
- `app/main.py` — lifespan, version, GZip
- `app/routes/admin.py` — SQL injection, Pydantic models
- `app/database.py` — CSV BOM, ADIF export, backup limit
- `app/adif_parser.py` — ADIF field parsing
- `app/backup.py` — backup retention
- `reset_password.py` — async compatibility

---

### Task 1: Fix SQL injection in stats/by-month
### Task 2: Replace deprecated @app.on_event with lifespan
### Task 3: Fix version number
### Task 4: Add GZip middleware
### Task 5: Fix CSV BOM encoding
### Task 6: Enhance ADIF parser fields
### Task 7: Enhance ADIF export
### Task 8: Add backup retention limit
### Task 9: Fix reset_password.py async compatibility
### Task 10: Add Pydantic request models (skip — low risk, high effort, manual JSON parsing works fine)
