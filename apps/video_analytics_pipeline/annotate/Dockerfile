FROM python:3.9
COPY requirements.txt requirements.txt

RUN pip install --upgrade pip
RUN pip install -r requirements.txt


COPY annotate_server.py annotate_server.py
COPY annotate.py annotate.py
COPY cfg cfg
CMD ["python", "annotate_server.py", "9000"]
