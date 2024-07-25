# ssh-data
A small data backend with data structures similar to redis using SSH for authentication and communication.

## State of the project
This project is currently in the planning phase with some code being written to test the feasibility of the project.

## Goals of this project
I mainly want to create a small data backend that I can use for my scripts and small applications.  
I want to use ssh for authentication and communication, because it is a well known and secure protocol and also supports features like multiplexing which makes for an easier implementation in quite a few cases.

## The Protocol
The protocol works over stdin and stdout and is line based with shell-like quoting/tokenization rules.  
The first token is always the command, followed by the arguments.  
Some commands output multiple answers delimited by newlines.  
The protocol also supports a more bot friendly mode in which the input and output are JSON objects.
This mode also uses newlines to separate answers, but the answers are JSON objects.  
The input is a JSON array with the first element being the command and the rest being the arguments.

## List of Commands
### Strings
A collection of string manipulation commands.
(Also supports a transaction command to do atomic client side operations (e.g. working with encrypted data structures))
### JSON
JSON is stored as a string, but can be manipulated using JSON modification commands.
### Streams
Streams can be used for pubsub, with different modes of operation.
- STREAMS CREATE \[options\] \<name\>
  - options:
    - --persistent/-p - make the stream persistent
    - --size/-s \<number\> - limit the size of the stream
    - --mpmc/-m - make the stream multi-producer multi-consumer (so messages are mapped one-to-one)
- STREAMS DELETE \<name\>
- STREAMS APPEND \<name\> \<data\>
- ...

## ToDo
- [ ] evaluate alternative design listed below
- [ ] implement PoC
- [ ] implement a locking mechanism to enable client-side json manipulation (to support encryption and similar features)
- [ ] add direct access to sqlite databases (perhaps over a json-rpc interface)
      the json-rpc interface should accept either raw sql or a json object with the sql query and its parameters
      the return value should be a json object `["success", <result_table>]` or `["error", "error message"]`.

## Alternative design
- have a single sqlite db per user that can be queried over a json-rpc interface and a repl
- also have support for "channels" that can either work via pubsub or mpmc to match each message one-to-one
- streams could then be implemented using a sqlite db + a message to a pubsub channel
- possibly use https://github.com/rqlite/sql to parse sql before encoding it again for sqlite to support channels and streams better?