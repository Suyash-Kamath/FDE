def main() -> None:
    import uvicorn

    uvicorn.run("pythondemo.main:app", host="0.0.0.0", port=8000)
