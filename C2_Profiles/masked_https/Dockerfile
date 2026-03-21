FROM itsafeaturemythic/mythic_go_base:latest

WORKDIR /Mythic/

COPY [".", "."]

RUN make build

CMD ["./mythic_go_services"]
