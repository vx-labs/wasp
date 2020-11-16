# <img width="48" height="38" src="assets/wasp-logo-1.svg"> Wasp

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fvx-labs%2Fwasp.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fvx-labs%2Fwasp?ref=badge_shield)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/9cbf68593140426591f86a7c744cc260)](https://app.codacy.com/gh/vx-labs/wasp?utm_source=github.com&utm_medium=referral&utm_content=vx-labs/wasp&utm_campaign=Badge_Grade)

Distributed MQTT broker, written in Go.

## Project status disclaimer

This project is a "personnal hobby playground" project.

It won't suit your needs if you are looking for an entreprise-grade MQTT Broker.

For example, Security, Release management, Automated feature and performance testing, Technical support (this list is obviously non-exhaustive)... are jobs that this project team does **NOT** do, or actively work on.

However, if you want to play and learn with Golang, MQTT, Gossip, GRPC, and other fun technologies, have fun with this project, and fell free to contribute by forking the project, hacking on it, or submiting a Pull Request !

## Installing

Wasp is currently released as a built binary file, you just have to download it, make it executable, and run it.

### Linux

If you are using Archlinux, an AUR package is available (named `wasp`).
For other Linux distributions, you can run the following shell one-liner:

```shell
curl -sLo Downloads/wasp https://github.com/vx-labs/wasp/releases/download/v4.0.0/wasp_linux-amd64 && chmod +x ./Downloads/wasp
```

Il will download Wasp, save it in your ./Downloads folder, and make it executable.

## Running Wasp on your workstation

Once downloaded, you can run Wasp by simply invoking it in your shell.

```shell
./Download/wasp
```

Wasp by default runs an MQTT listener on port 1883, and will print on the terminal the IP adfress ip it detected.
You can connect to it using an MQTT client, like Mosquitto.

```shell
mosquitto_sub -t 'test' -d -q 1 &
mosquitto_pub --port 1884 -t 'test' -d -q 1 -m 'hello'
```

### Increasing log level

Wasp use a leveled logger, supporting traditional level name.

* debug
* info
* warning
* error

It will by default only print `warning` and `error` messages.
You can alter this behaviour by adding the `log-level` flag when invoking Wasp.

For example, if you want to display `info` messages and above, you can type:

```shell
./Downloads/wasp --log-level info
```

### Persisting data between restarts

Wasp builds a _State_ to store all retained messages, subscriptions, session metadatas, ... etc.
This state is saved on a filesystem, by the default Wasp configuration is to save it in `/tmp/wasp`.

This folder is, in most workstation configuration, erased at startup.

If you want your Wasp data to survive a host reboot, you must change this location by passing the `data-dir` flag.

For example, if you want to save Wasp data in the `wasp-data` folder, you can run the following command.

```shell
./Downloads/wasp --data-dir ./wasp-data
```
