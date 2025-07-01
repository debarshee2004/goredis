package main

import (
	"fmt"
	"log/slog"

	"github.com/tidwall/resp"
)

/*
handleMessage processes incoming command messages from clients

This is the central message processing function that gets called whenever
a client sends a command to the server. It follows this flow:

1. Execute the command using the storage engine
2. Handle any execution errors by sending error responses
3. Format successful results according to RESP protocol
4. Send the response back to the client
5. Log any communication errors

The function must handle different types of command results:
  - Simple values (strings, numbers)
  - Complex RESP-formatted data (arrays, maps)
  - Null responses (when keys don't exist)
  - Error conditions

Parameters:
  - msg: Contains the parsed command and the peer (client) that sent it

Returns: error if there was a communication problem (not command errors)

Error Handling Philosophy:
  - Command errors (like "key not found") are sent to the client as RESP errors
  - Communication errors (network issues) are returned as Go errors
  - This separation allows the server to stay running even when individual commands fail
*/
func (s *Server) handleMessage(msg Message) error {
	/*
		Execute the command using the storage engine
		This calls the Execute method on the Command interface
		The storage engine performs the actual operation (GET, SET, etc.)
	*/
	result, err := msg.cmd.Execute(s.storage)

	/*
		Handle command execution errors
		These are logical errors like "key not found" or "wrong type"
		They should be reported to the client, not crash the server
	*/
	if err != nil {
		/*
			Format error message according to Redis conventions
			Redis error messages start with "ERR " followed by the description
		*/
		errorMsg := fmt.Sprintf("ERR %s", err.Error())

		/*
			Send error response to client using RESP protocol
			RESP errors start with "-" and end with "\r\n"
		*/
		if writeErr := resp.NewWriter(msg.peer.connect).WriteError(fmt.Errorf(errorMsg)); writeErr != nil {
			slog.Error("failed to write error response", "err", writeErr)
			return writeErr
		}
		return nil
	}

	/*
		Send successful response to client
		The response format depends on what the command returned
	*/
	if result != nil {
		/*
			Check if result is already formatted as RESP data
			Some commands (like MGET, KEYS) return pre-formatted RESP responses
			Others return raw data that needs to be wrapped in RESP format
		*/
		if isRESPFormatted(result) {
			/*
				Send raw RESP data directly to client
				This is used for complex responses like arrays and maps
			*/
			_, writeErr := msg.peer.Send(result)
			if writeErr != nil {
				slog.Error("failed to send RESP response", "err", writeErr)
				return writeErr
			}
		} else {
			/*
				Send as bulk string (most common case)
				RESP bulk strings: $<length>\r\n<data>\r\n
				Example: $5\r\nhello\r\n for the string "hello"
			*/
			if writeErr := resp.NewWriter(msg.peer.connect).WriteBytes(result); writeErr != nil {
				slog.Error("failed to write bulk response", "err", writeErr)
				return writeErr
			}
		}
	} else {
		/*
			Send null response for commands that return nil
			This happens when GET is called on a non-existent key
			RESP null: $-1\r\n
		*/
		if writeErr := resp.NewWriter(msg.peer.connect).WriteNull(); writeErr != nil {
			slog.Error("failed to write null response", "err", writeErr)
			return writeErr
		}
	}

	return nil
}

/*
isRESPFormatted checks if the byte slice contains RESP formatted data

RESP (Redis Serialization Protocol) uses specific prefix characters to indicate
data types. This function checks if data is already in RESP format so we know
whether to send it directly or wrap it in a bulk string.

RESP Data Type Prefixes:
  - '+' Simple String: +OK\r\n
  - '-' Error: -ERR something went wrong\r\n
  - ':' Integer: :42\r\n
  - '$' Bulk String: $5\r\nhello\r\n
  - '*' Array: *2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
  - '%' Map: %2\r\n+key1\r\n+value1\r\n+key2\r\n+value2\r\n (Redis 6.0+)

Parameters:
  - data: The byte slice to check

Returns: true if data starts with a RESP prefix character, false otherwise

Usage Examples:
  - isRESPFormatted([]byte("+OK\r\n")) -> true (simple string)
  - isRESPFormatted([]byte("hello")) -> false (raw data, needs bulk string wrapper)
  - isRESPFormatted([]byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n")) -> true (array)

Why This Matters:
  - Pre-formatted RESP data should be sent as-is to maintain correct protocol structure
  - Raw data needs to be wrapped in bulk string format for protocol compliance
  - Mixing formatted and unformatted data would break the RESP protocol
*/
func isRESPFormatted(data []byte) bool {
	// Empty data is not considered RESP formatted
	if len(data) == 0 {
		return false
	}

	// Check first character against known RESP prefixes
	firstChar := data[0]
	return firstChar == '+' ||
		firstChar == '-' ||
		firstChar == ':' ||
		firstChar == '$' ||
		firstChar == '*' ||
		firstChar == '%'
}
