package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

func (s *UserServer) Start(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	write := bufio.NewWriter(out)
	for scanner.Scan() {
		var commandList []interface{}
		err := json.Unmarshal(scanner.Bytes(), &commandList)
		if err != nil {
			s.printErrorWithMessage(write, "Could not unmarshal command", err)
			continue
		}
		if len(commandList) == 0 {
			s.printErrorMessage(write, "empty command")
			continue
		}
		command, ok := commandList[0].(string)
		if !ok {
			s.printErrorMessage(write, "Invalid type of command")
			continue
		}
		switch command {
		case "sql":
			if len(commandList) < 2 {
				s.printErrorMessage(write, "Invalid number of arguments")
				continue
			}
			query, ok := commandList[1].(string)
			if !ok {
				s.printErrorMessage(write, "Invalid type of argument")
				continue
			}
			args := make([]interface{}, len(commandList)-2)
			for i := 2; i < len(commandList); i++ {
				args = append(args, commandList[i])
			}
			resp, err := s.userDB.queryToJSON(query, args...)
			if err != nil {
				s.printError(write, err)
				continue
			}
			fmt.Println(resp)
		case "pub":
			if len(commandList) != 3 {
				s.printErrorMessage(write, "Invalid number of arguments")
				continue
			}
			channel, ok := commandList[1].(string)
			if !ok {
				s.printErrorMessage(write, "Invalid type of argument")
				continue
			}
			message, ok := commandList[2].(string)
			if !ok {
				s.printErrorMessage(write, "Invalid type of argument")
				continue
			}
			ch := s.userDB.GetChannel(channel)
			ch <- message
		case "sub":
			if len(commandList) != 2 {
				s.printErrorWithMessage(write, "Invalid number of arguments", fmt.Errorf("command: %v", command))
				continue
			}
			channel, ok := commandList[1].(string)
			if !ok {
				s.printErrorMessage(write, "Invalid type of argument")
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
					continue
				}
			}
		case "end":
			return nil
		default:
			s.printErrorMessage(write, "Invalid command")
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		s.logger.Error("Error reading from stdin", "error", err)
		return fmt.Errorf("error reading from stdin: %w", err)
	}
	return nil
}

func (s *UserServer) Close() error {
	return s.userDB.Close()
}

func (s *UserServer) printErrorMessage(out io.Writer, message string) {
	s.printError(out, fmt.Errorf(message))
}

func (s *UserServer) printErrorWithMessage(out io.Writer, message string, err error) {
	message = fmt.Sprintf("%s: %s", message, err)
	s.printError(out, fmt.Errorf(message))
}

func (s *UserServer) printError(out io.Writer, err error) {
	message := []string{"error", err.Error()}
	messageJSON, err := json.Marshal(message)
	if err != nil {
		s.logger.Error("Could not marshal message", "error", err)
		return
	}
	_, err = out.Write(messageJSON)
	if err != nil {
		s.logger.Error("Could not write message", "error", err)
	}
}
