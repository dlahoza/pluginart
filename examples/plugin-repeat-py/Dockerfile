FROM python:3.12-slim
WORKDIR /app
COPY runtimes/python /runtime
COPY examples/plugin-repeat-py /app
RUN pip install --no-cache-dir /runtime -r requirements.txt
ENTRYPOINT ["python", "plugin.py"]
