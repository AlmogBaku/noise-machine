# Noise Machine

A simple and effective noise machine built to soothe babies and dogs, featuring a friendly, yet plain, web UI. While
this project can run on various platforms, this guide focuses on deploying it to a Raspberry Pi connected to a speaker.

## Overview

This noise machine was created to help relax my dog and reduce nighttime disturbances (she can be quite barky...). It's
particularly effective in masking outside noises, which can significantly decrease a dog's barking and improve sleep
quality for both pets, their owners, and the owner's health by not waking up their baby daughters 🤪.

That's been said, it can be quite useful for a variety of tasks, such as soothing your babies, or even to get a good
sleep.

## Features

- Web-based interface for easy control
- Start/stop functionality
- Volume control
- Automatic download of noise file (default is a calming brook sound)
- Can be controlled via HTTP requests for easy automation

## Development Requirements

To develop and build this project, you need:

- Go programming language installed on your development machine
- Basic understanding of Go and web development

## Building the Project

1. Clone this repository to your development machine.
2. Navigate to the project directory.
3. Build the project for your target platform. For Raspberry Pi, use:

```bash
GOOS=linux GOARCH=arm GOARM=6 go build -o noiseapp
```

## Deployment

### Target Device Dependencies

The noise machine requires the following dependencies on the target device (e.g., Raspberry Pi):

- `play` command (part of the `sox` package)
- `amixer` for volume control (part of the `alsa-utils` package)

### Installing Dependencies on Raspberry Pi

To install the required dependencies on a Raspberry Pi:

1. Update the package list:
   ```
   sudo apt update
   ```

2. Install Sox (for the `play` command):
   ```
   sudo apt install sox libsox-fmt-all
   ```

3. Install ALSA utilities (for the `amixer` command):
   ```
   sudo apt install alsa-utils
   ```
4. Install cURL and Cron - for automation:
   ```
   sudo apt install curl cron
   ```

### Deploying the Application

1. After building the application on your development machine, copy it to the Raspberry Pi:

```bash
scp noiseapp pi@raspberrypi.local:~
```

*Notice:* When running as a service, make sure to stop it before updating the file

2. SSH into your Raspberry Pi and move the application to its intended directory:

```bash
ssh pi@raspberrypi.local
sudo mv ~/noiseapp /opt/noiseapp/
sudo chmod +x /opt/noiseapp/noiseapp
```

## Usage

1. On the Raspberry Pi, run the application:

```bash
/opt/noiseapp/noiseapp
```

2. Access the web interface by navigating to `http://<raspberry-pi-ip>:8888` in a browser.

3. Use the web interface to start/stop the noise and adjust the volume.

## Automation

### Using crontab

Crontab can be used to automatically start and stop the noise machine at specific times. Add the following to the
crontab (`crontab -e`):

```
0 22 * * * curl -X GET "http://localhost:8888/start"
30 23 * * * curl -X GET "http://localhost:8888/start"
30 7 * * * curl -X GET "http://localhost:8888/stop"
```

This starts the noise at 10:00 PM and 11:30 PM, and stops it at 7:30 AM. Adjust these times as needed.

### Running as a systemd service

To run the noise machine as a systemd service:

1. Create a file named `noise.service` in `/etc/systemd/system/` with the following content:

```ini
[Unit]
Description=Noise Machine

[Service]
ExecStart=/opt/noiseapp/noiseapp
WorkingDirectory=/opt/noiseapp/
Restart=always
User=pi
Group=pi
Environment=PATH=/usr/bin:/usr/local/bin

[Install]
WantedBy=multi-user.target
```

2. Reload systemd:

```bash
sudo systemctl daemon-reload
```

3. Enable and start the service:

```bash
sudo systemctl enable noise.service
sudo systemctl start noise.service
```