FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
	CGO_ENABLED=0 GOOS=linux \
	go build -trimpath -ldflags="-s -w" -o /out/ssh-server .

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache --virtual .keygen openssh-keygen \
	&& mkdir -p .ssh \
	&& ssh-keygen -t ed25519 -f .ssh/id_ed25519 -N "" \
	&& chmod 600 .ssh/id_ed25519 \
	&& apk del .keygen

COPY --from=build /out/ssh-server /app/ssh-server

RUN chown -R nobody:nobody /app

EXPOSE 23234

USER nobody
CMD ["/app/ssh-server"]
