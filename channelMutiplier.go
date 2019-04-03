// Copyright 2019 Michael Mitchell
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"reflect"
	"sync"
)

// Multiplier will multiply the source channels content to the destination channels. Data written to the Source
// Channel will need to be type-asserted back to the correct type when received from a destination channel.
// Destination channels can be added dynamically at any point during the life of a Multiplier.
type Multiplier struct {
	SourceChannel       interface{}
	destinationChannels []chan interface{}
	lock                sync.Mutex
}

// NewMultiplier is a simple constructor to create a Multiplier
func NewMultiplier(sourceChannel interface{}) Multiplier {
	if value := reflect.ValueOf(sourceChannel) ; value.Kind() == reflect.Chan {
		return Multiplier{
			SourceChannel: sourceChannel,
		}
	}
	panic("SourceChannel is not a channel!")
}

// RegisterChannel allows adding a destination channel that should be written to when data is written to the
// SourceChannel.
func (mult *Multiplier) RegisterChannel(ch chan interface{}) {
	if value := reflect.ValueOf(ch) ; value.Kind() == reflect.Chan {
		mult.lock.Lock()
		mult.destinationChannels = append(mult.destinationChannels, ch)
		mult.lock.Unlock()
	} else {
		panic("ch is not a channel!")
	}
}

// RegisterChannels allows adding an array of destination channels that should be written to when data is written
// to the SourceChannel.
func (mult *Multiplier) RegisterChannels(ch []chan interface{}) {
	for i := range ch {
		mult.RegisterChannel(ch[i])
	}
}

// ChannelGenerator is a closure that will return pre-registered channels that will receive
// values written to SourceChannel!
func (mult *Multiplier) ChannelGenerator() func(channelLen int) chan interface{} {
	return func(channelLen int) chan interface{} {
		newChannel := make(chan interface{}, channelLen)
		mult.RegisterChannel(newChannel)
		return newChannel
	}
}

// Multiply is designed to be called asynchronously as it blocks. Multiply will wait for data to be received from
// SourceChannel, then start threads to write that data to the destination channels created with ChannelGenerator.
func (mult *Multiplier) Multiply() {
	channel := reflect.ValueOf(mult.SourceChannel)
	for {
		x, ok := channel.Recv()
		if ok {
			mult.lock.Lock()
			for _, ch := range mult.destinationChannels {
				ch := ch
				go func (channel chan interface{}) {
					channel <- x.Interface()
				} (ch)
			}
			mult.lock.Unlock()
		} else {
			return
		}
	}
}
