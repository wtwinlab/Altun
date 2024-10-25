package main

type Key int

type Value []byte

type Command struct {
	Key       Key
	Value     Value
	ClientID  ID
	CommandID int
}
