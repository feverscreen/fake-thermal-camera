# Build:                   sudo docker build --no-cache . -t cacophony-api
# Run interactive session: sudo docker run -it cacophony-api

FROM golang:latest

RUN apt-get update
RUN apt-get install -y apt-utils
RUN apt-get install -y supervisor
RUN apt-get install -y dbus
RUN mkdir -p /var/run/dbus

RUN mkdir -p /var/log/supervisor
# server for automated testing
EXPOSE 2040
EXPOSE 80
    
COPY  fever-screen.conf /etc/supervisor/conf.d/fever-screen.conf
COPY  thermal-uploader.conf /etc/supervisor/conf.d/thermal-uploader.conf
COPY  event-reporter.conf /etc/supervisor/conf.d/event-reporter.conf


# WORKDIR /server
# RUN ls
# RUN go get ./...
# RUN go build ./...
# RUN testing-server
COPY  docker-entrypoint.sh /
RUN mkdir /etc/cacophony
RUN mkdir /var/spool/cptv

COPY recorder-config.toml /etc/cacophony/config.toml

ENTRYPOINT ["/docker-entrypoint.sh"]
