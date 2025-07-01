package main

import (
	"fmt"
	"log/slog"

	"github.com/tidwall/resp"
)

func (s *Server) handleMessage(msg Message) error {
	result, err := msg.cmd.Execute(s.storage)

	if err != nil {
		errorMsg := fmt.Sprintf("ERR %s", err.Error())

		if writeErr := resp.NewWriter(msg.peer.connect).WriteError(fmt.Errorf(errorMsg)); writeErr != nil {
			slog.Error("failed to write error response", "err", writeErr)
			return writeErr
		}
		return nil
	}

	if result != nil {
		if isRESPFormatted(result) {
			_, writeErr := msg.peer.Send(result)
			if writeErr != nil {
				slog.Error("failed to send RESP response", "err", writeErr)
				return writeErr
			}
		} else {
			if writeErr := resp.NewWriter(msg.peer.connect).WriteBytes(result); writeErr != nil {
				slog.Error("failed to write bulk response", "err", writeErr)
				return writeErr
			}
		}
	} else {
		if writeErr := resp.NewWriter(msg.peer.connect).WriteNull(); writeErr != nil {
			slog.Error("failed to write null response", "err", writeErr)
			return writeErr
		}
	}

	return nil
}

func isRESPFormatted(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	firstChar := data[0]
	return firstChar == '+' ||
		firstChar == '-' ||
		firstChar == ':' ||
		firstChar == '$' ||
		firstChar == '*' ||
		firstChar == '%'
}
