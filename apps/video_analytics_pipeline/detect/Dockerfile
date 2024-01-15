FROM python:3.9
COPY requirements.txt requirements.txt

RUN pip install --upgrade pip
RUN pip install -r requirements.txt


COPY detect_server.py detect_server.py
COPY detect.py detect.py
COPY cfg cfg
CMD ["python", "detect_server.py", "9000"]
