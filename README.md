# Matrix-EmailBridge
A matrix-bridge written in Go to let you read and write your emails in matrix. You can have multiple email accounts in different private rooms and write emails to one or multiple recipients.

## Installation
### Compile method
Clone this repository and run inside the folder
```bash
go get -v
go build -o emailbridge
./emailbridge
```
The last command executes the bridge once to create the probably missing config file.<br>
--> [Configure](https://github.com/JojiiOfficial/Matrix-EmailBridge#Get-started)

### Docker method
DockerHub: https://hub.docker.com/repository/docker/jojii/matrix_email_bridge<br><br>
Run 
```bash
docker pull jojii/matrix_email_bridge
```
to pull the image. Then create a container by running
```bash
docker run -d \
--restart unless-stopped \
-v `pwd`/data:/app/data \
--name email_bridge \
jojii/matrix_email_bridge
```
<br>
This will create and start a new Docker Container and create a new dir called 'data' in the current directory. In this folder data.db, cfg.json and the logs will be stored.<br>

After [configuring the bridge](https://github.com/JojiiOfficial/Matrix-EmailBridge#Get-started) you have to run
```bash
docker start email_bridge
```
to start the docker container again.
<br>
`
Note: 'localhost' as 'matrixserver' (in cfg.json) wouldn't work because of dockers own network. You have to specify the internal IP address of the matrix-synapse server!
`

# Get started
1. Create a bot user.
2. Adjust the config file (cfg.json) to make it work with your matrix server.
3. Invite your bot into a private room, it will join automatically.<br>

If everything is set up correctly, you can bridge the room by typing <code>!login</code>. Then you just have to follow the instructions. The command <code>!help</code> shows a list with available commands.<br>
Creating a new private room with the bot/bridge lets you add a different email account.<br>


## Note
Note: you should change the permissions of the <code>cfg.json</code> and <code>data.db</code> to <b>640</b> or <b>660</b> because they contain sensitive data.

## Features
- [X]  Receiving Email with IMAPs
- [X]  Use custom IMAPs Server and port
- [X]  Use the bridge with multiple email addresses
- [X]  Use the bridge with multiple user
- [X]  Ignore SSL certs if required
- [X]  Detailed error codes/logging 
- [X]  Use custom mailbox instead of INBOX
- [X]  Sending emails (to one or multiple participants)
- [X]  Use markdown (automatically translated to HTML) for writing emails (optional)
- [X]  Viewing HTML messages (as good as your matrix-client supports html)
- [X]  Attaching files sent into the bridged room
- [X]  Emailaddress blocklist (Ignore emails from given emailaddress)

## TODO

- [ ]  System to send passwords not in plaintext
- [ ]  Add more header (CC/Bcc)
- [ ]  Update the installerscript
