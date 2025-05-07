FROM golang:alpine

# Install basic dependencies
RUN apk update  &&  apk upgrade
RUN apk add --update --no-interactive \
	btop htop

# Creates an app directory to hold your app’s source code
WORKDIR /app

# Copies everything from your root directory into /app
COPY --link . .

# Installs Go dependencies
RUN go mod download

# Add test user for the application
RUN addgroup www  &&  adduser --disabled-password --ingroup www www

# Debug run
CMD [ "go", "run", ".", "-c", "simon.example.yml", "-i", "1" ]


# Builds your app with optional configuration
#RUN go build .
#RUN go build -ldflags "-s -w" .

# Run
#CMD [ "/app/simon", "-c", "simon.example.yml", "-i", "2" ]
