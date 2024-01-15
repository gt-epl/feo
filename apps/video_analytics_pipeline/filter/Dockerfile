FROM python:3.9
COPY requirements.txt requirements.txt

RUN pip install --upgrade pip
RUN pip install -r requirements.txt


COPY filter_server.py filter_server.py
COPY filter.py filter.py
CMD ["python", "filter_server.py", "9000"]
