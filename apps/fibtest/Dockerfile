FROM openwhisk/actionloop-base
COPY fibtest.c fibtest.c
COPY Makefile Makefile
RUN apt update && apt -y install build-essential
RUN make


