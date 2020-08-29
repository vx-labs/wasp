# Wasp

Distributed MQTT broker, written in Go.

## Project status disclaimer

This project is a "personnal hobby playground" project.

It won't suit your needs if you are looking for an entreprise-grade MQTT Broker.

For example, Security, Release management, Automated feature and performance testing, Technical support (this list is obviously non-exhaustive)... are jobs that this project team does **NOT** do, or actively work on.

However, if you want to play and learn with Golang, MQTT, Raft, GRPC, and other fun technologies, have fun with this project, and fell free to contribute by forking the project, hacking on it, or submiting a Pull Request !

## Installing

Wasp is currently released as a built binary file, you just have to download it, make it executable, and run it.

### Linux

If you are using Archlinux, an AUR package is available (named `wasp`).
For other Linux distributions, you can run the following shell one-liner:

```
curl -sLo Downloads/wasp https://github.com/vx-labs/wasp/releases/download/v1.1.2/wasp_linux-amd64 && chmod +x ./Downloads/wasp
```
Il will download Wasp, save it in your ./Downloads folder, and make it executable.

### MacOS

Packaging software for MacOS is traditionnaly done using a .dmg file, or via the MacAppStore, however I do not have the resources nor the knowledge to do it, so we have to stick with the same curl-chmod process we use on Linux.

```
curl -sLo Downloads/wasp https://github.com/vx-labs/wasp/releases/download/v1.1.2/wasp_darwin-amd64 && chmod +x ./Downloads/wasp
```

## Running Wasp on your workstation

Once downloaded, you can run Wasp by simply invoking it in your shell.

```
./Download/wasp
```

Wasp by default runs an MQTT listener on port 1883, and will print on the terminal the IP adfress ip it detected.
You can connect to it using an MQTT client, like Mosquitto.

```
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

```
./Downloads/wasp --log-level info
```

### Persisting data between restarts

Wasp build a _State_ to store all retained messages, subscriptions, session metadatas, ... etc.
This state is saved on a filesystem, by the default Wasp configuration is to save it in `/tmp/wasp`.

This folder is, in most workstation configuration, erased at startup.

If you want your Wasp data to survive a host reboot, you must change this location by passing the `data-dir` flag.

For example, if you want to save Wasp data in the `wasp-data` folder, you can run the following command.

```
./Downloads/wasp --data-dir ./wasp-data
```


### Troubleshooting common problems

#### `panic: removed all voters`

Wasp can refuse to start, only printing the following message.

```
panic: removed all voters

goroutine 32 [running]:
go.etcd.io/etcd/raft.(*raft).applyConfChange(0xc0002a4000, 0x0, 0xc000132570, 0x1, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, ...)
	go.etcd.io/etcd@v0.0.0-20200319002442-e784ba73c229/raft/raft.go:1514 +0x197
go.etcd.io/etcd/raft.(*node).run(0xc0000be7e0)
	go.etcd.io/etcd@v0.0.0-20200319002442-e784ba73c229/raft/node.go:356 +0x78c
created by go.etcd.io/etcd/raft.RestartNode
	go.etcd.io/etcd@v0.0.0-20200319002442-e784ba73c229/raft/node.go:240 +0x33d
```

This message basically says that Wasp state became corrupted, and could not be recovered at startup.

This is a bug, and it will certainly be fixed in the future, but you might encounter it.

It often happens when you attempt to kill Wasp before it started.

You must erase the current Wasp state to allow Wasp to start again (You will loose all retained messages. If this is a problem to you, you might want to look at Wasp clustering capabilities).

For example, if you use the default Wasp configuration, data is stored in /tmp/wasp, and you can erase it using the following command.

```
rm -rf /tmp/wasp
```
