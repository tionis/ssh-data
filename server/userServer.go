package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type UserServer struct {
	ctx    context.Context
	userDB *UserDB
	logger *slog.Logger
}

func NewUserServer(logger *slog.Logger, context context.Context, dbPath string) (*UserServer, error) {
	userDB, err := NewUserDB(context, dbPath, logger)
	if err != nil {
		logger.Error("Could not open database", "error", err)
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	return &UserServer{userDB: userDB, logger: logger, ctx: context}, nil
}

func (s *UserServer) Start() error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var commandList []interface{}
		err := json.Unmarshal(scanner.Bytes(), &commandList)
		if err != nil {
			s.logger.Error("Could not unmarshal command", "error", err)
			continue
		}
		if len(commandList) == 0 {
			s.logger.Error("Empty command")
			continue
		}
		command, ok := commandList[0].(string)
		if !ok {
			s.logger.Error("Invalid command", "command", commandList)
			continue
		}
		switch command {
		case "sql":
			if len(commandList) < 2 {
				s.logger.Error("Invalid number of arguments", "command", command)
				continue
			}
			query, ok := commandList[1].(string)
			if !ok {
				s.logger.Error("Invalid type of argument", "command", command)
				continue
			}
			args := make([]interface{}, len(commandList)-2)
			for i := 2; i < len(commandList); i++ {
				args = append(args, commandList[i])
			}
			resp, err := s.userDB.queryToJSON(query, args...)
			if err != nil {
				s.logger.Error("Could not execute query", "error", err)
				continue
			}
			fmt.Println(resp)
		case "pub":
			if len(commandList) != 3 {
				s.logger.Error("Invalid number of arguments", "command", command)
				continue
			}
			channel, ok := commandList[1].(string)
			if !ok {
				s.logger.Error("Invalid type of argument", "command", command)
				continue
			}
			message, ok := commandList[2].(string)
			if !ok {
				s.logger.Error("Invalid type of argument", "command", command)
				continue
			}
			ch := s.userDB.GetChannel(channel)
			ch <- message
		case "sub":
			if len(commandList) != 2 {
				s.logger.Error("Invalid number of arguments", "command", command)
				continue
			}
			channel, ok := commandList[1].(string)
			if !ok {
				s.logger.Error("Invalid type of argument", "command", command)
				continue
			}
			ch := s.userDB.GetChannel(channel)
			for {
				select {
				case <-s.ctx.Done():
					return nil
				case message := <-ch:
					messageJSON, err := json.Marshal(message)
					if err != nil {
						s.logger.Error("Could not marshal message", "error", err)
						continue
					}
					fmt.Println(string(messageJSON))
				}
			}
		case "end":
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		s.logger.Error("Error reading from stdin", "error", err)
		return fmt.Errorf("error reading from stdin: %w", err)
	}
	return nil
}
