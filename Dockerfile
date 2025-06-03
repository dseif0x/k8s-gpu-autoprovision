FROM alpine:latest

RUN apk add --no-cache curl jq kubectl bash

COPY gpu-check.sh /gpu-check.sh
RUN chmod +x /gpu-check.sh

CMD ["/gpu-check.sh"]
