FROM python:alpine3.7
COPY fibtest.c fibtest.c
COPY Makefile Makefile
COPY fibtest_server.py fibtest_server.py
RUN apk update && apk add --virtual build-dependencies build-base gcc make
RUN make
CMD ["python", "fibtest_server.py", "9000"]

