# Hakobiya
The real-time helper. Remote data binding for Angular.js using Websockets.

Do you have an old app using AJAX for 'soft' real time and need a really real-time bridge? Do you have an app that just needs to push around some data from client to client and you don't want to write a whole new server? Hakobiya could be the answer.

A Hakobiya server has one or more **channel** templates. Channels have names, and the type of channel is specified by the first letter of the name. You can configure each type of channel to store **variables** (data for each user), computed values of user variables (**magic variables**), and client to client communication slots called **wires**. Additionally, you can chose to expose **system variables** to clients, such as the number of clients listening in a particular channel. You can then bind any of these values to Javascript variables and everything will be automatically synchronized.

Hakobiya has an HTTP API that lets you set values and send messages in any channel, so you can have something else handle the logic and let Hakobiya take care of updating your users.

# Configuration 
The default config file is `config.toml`. You can specify a different file with the `-config` flag:
```
hakobiya -config /some/dir/hakobiya.toml
```

#### Types
Variable definitions often require a **type**, which can be any of the following:

* `"bool"`
* `"int"`
* `"float"`
* `"string"`
* `"object"`
* `"any"`

## Server config
``[server]`` 

General server configuration.

| Name | Type   | Required?    | Default       | Description                 |
| ---- | ------ | ------------ | ------------- | --------------------------- |
| name | string | *optional*   | `"Hakobiya"`  | Server name                 |
| bind | string | *optional*   | `":8080"`     | Bind address: `[host]:port` |
| path | string | *optional*   | `"/hakobiya"` | Path for Websocket server   |

#### Example
Sets up a server called Chat Helper at `ws://0.0.0.0/chat`. 
```toml
[server]
name = "Chat Helper"
bind = ":80"
path = "/chat"
```

## API config
`[api]` 

The API lets you send messages to clients and set and get values via HTTP.

| Name    | Type   | Required?  | Default  | Description                 |
| ------- | ------ | ---------- | -------- | --------------------------- |
| enabled | bool   | *optional* | `false`  | Enable the API?             |
| path    | string | *optional* | `"/api"` | API root path               |
| key     | string | *optional* |          | Secret key                  |

#### Example
Enables the API at the default path with a secret key of `"turtles"`.
```toml
[api]
enabled = true
key = "turtles"
```

## Channel config
`[[channel]]` 

Defines channels (note the double brackets). Channels are distinguished by their first letter (the `prefix` value) and are created on-the-fly when joined by a client. Any string after the prefix is OK for a channel name.

| Name   | Type     | Required?    | Default  | Description                 |
| ------ | -------- | ------------ | -------- | --------------------------- |
| prefix | char     | **required** |          | Distinguishing prefix       |
| expose | string[] | *optional*   | `[]`     | System variables to expose  |

#### Example
Defines a channel with a prefix of `"c"` that exposes the system variable ``$listeners`` to clients. Any channel with a name starting with "c" will be handled by this: `c123`, `cTest`, etc.
```toml
[[channel]]
prefix = "c"
expose = ["$listeners"]
```

### User variables (%var)
`[channel.var.(variable name)]`

Defines a per-user value. You can use these values when computing magic variables.

| Name        | Type | Required?  | Default   | Description                      |
| ----------- | ---- | ---------- | --------- | -------------------------------- |
| type        | type | *optional* | `"any"`   | The type of this variable        |
| ~~default~~ | *    | *optional* |           | Default value for this variable  |

#### Example
Defines a string user variable called `%username`, and a boolean variable called `%typing`.
```toml
[channel.var.username]
	type = "string"
[channel.var.typing]
    type = "bool"
```

### Magic, computed values (&var)
`[channel.magic.(variable name)]`

Magic variables are computed values based on user variables.

| Name | Type   | Required?    | Default | Description                                                   |
| ---- | ------ | ------------ | ------- | ------------------------------------------------------------- |
| src  | string | **required** |         | Source variable to base calculations on                       |
| func | string | **required** |         | Name of the function to reduce the values, see the Magic docs |

#### Example
Defines a magic variable called `&typers` that counts the number of users who have `%typing` set to `true`.
```toml
[channel.magic.typers]
	src  = "%typing"
	func = "count"
```

### Wire (=var)
`[channel.wire.(variable name)]`

Wires let clients send and receive messages to all other clients on the channel. You can optionally specify rules to rewrite incoming messages, combining them with other variables or literal text. You can make a wire read-only, which prevents clients from sending messages to it but allows you to send messages via the HTTP API.

| Name        | Type | Required?  | Default   | Description                                         |
| ----------- | ---- | ---------- | --------- | --------------------------------------------------- |
| type        | type | *optional* | `"any"`   | Input type  										|
| readonly    | bool | *optional* | `"false"` | If set to true, only the HTTP API can send messages |

#### Wire rewrite rules 
`[channel.wire.(variable name).rewrite]`

A table used for compositing input and other variables or literal text. The keys are the names for the new JSON object fields, the values are variable names (with sigil) to substitute (such as `"%name"`), `"$input"` to specify the input, or text literals with a leading single quote (like `"'hello"`).

#### Example
Defines a wire called `=chat` that takes a string as input and rewrites it as an object containing the input and the username of the sender.
```toml
[channel.wire.chat] 
	type = "string"
	[channel.wire.chat.rewrite]
		sender = "%username" 
		msg = "$input"
```
A client could send a message like `"hello world"` to the wire, which would then send a JSON object like this to all clients on the channel:
```json
{
    "sender": "LlamaGuy",
    "msg": "hello world"
}
```

# Hakobiya.js
Angular.js module. Include the `hakobiya` module in your project and use `Hakobiya.bind()` to do your dirty work.
```javascript
function ChatCtrl($scope, Hakobiya) {
	$scope.channel = "c12345";
	$scope.name = "Guest";
	$scope.chat = undefined;
	$scope.users = 0;

	Hakobiya.bind($scope, "channel",
	{
		"name": "%username",
		"chat": "=chat",
		"users": "$listeners",
	});
}
```

**User variables** have two-way binding. **System variables** have one-way binding. **Broadcasts** have one-way binding and are represented as an array. **Wires** are a special array: they have a `.send()` method to send data. 

You can manually listen for changes to any of these variables like so:
```javascript
$scope.$on("c12345.=chat", function(evt, value) {
    console.log(value);
});
```
Event names have the format of `(channel).(variable)`.

Also, don't forget to connect.
```javascript
myModule.run(function (Hakobiya) {
				Hakobiya.connect("ws://0.0.0.0/chat");
			});
```

# HTTP API
You can choose to expose an HTTP API, docs soon.