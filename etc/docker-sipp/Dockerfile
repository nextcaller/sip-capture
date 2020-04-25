FROM debian:buster-slim

RUN apt-get update && apt-get install -y sip-tester

CMD ["/usr/bin/sipp", "-f", "60", "-nostdin", "-sn", "uas", "-t", "un", "-i", "127.0.0.1"]
