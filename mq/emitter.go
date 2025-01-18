package mq

import "fmt"

func Emit(eventName string) error {
	var err error
	fmt.Println(eventName, " emitted")
	return err
}
