socketwrench, herein referred to as SW, is quite dumb. It takes orders from core through a redis
Pub/Sub of configurable name. Every message sent over this Pub/Sub begins with one byte,
interpreted as a uint8, for the packet identifier. Core should ignore packets with an ID less than
128, and SW will ignore packets with an ID greater or equal to 128.

## messages to SW from core

ID 0:
	Global message to all connected websockets. SW will send the remaining contents of
	the packet to all connected websockets.
ID 1:
	User message. SW will parse the next 4 bytes of the packet as a big-endian uint32, then
	send the rest of the packet to all websockets authenticated with that number as user ID.
ID 2:
	Token verification failed. The rest of the packet will be interpreted as a token string,
	and SW will eliminate that token/websocket pair from the waiting list.
ID 3:
	Token verification successful. SW will parse the next 4 bytes of the packet as a big-endian
	uint32, then the rest of the packet as a token string. The connection that sent that token
	will then be authenticated as the user with ID of that uint32.


## messages from SW to core

ID 128:
	Request to authenticate. SW will include a token string directly after this identifier, and
	expects backend to validate the token and send back either packet ID 2 or 3.