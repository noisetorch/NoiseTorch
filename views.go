package main

import (
	"fmt"
	"sync"

	"github.com/aarzilli/nucular"
)

type ViewFunc func(ctx *ntcontext, w *nucular.Window)

type ViewStack struct {
	stack [100]ViewFunc
	sp    int8
	mu    sync.Mutex
}

func NewViewStack() *ViewStack {
	return &ViewStack{sp: -1}
}

func (v *ViewStack) Push(f ViewFunc) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stack[v.sp+1] = f
	v.sp++
}

func (v *ViewStack) Pop() (ViewFunc, error) {
	v.mu.Lock()

	if v.sp <= 0 {
		return nil, fmt.Errorf("Cannot pop root element from ViewStack")
	}

	defer (func() {
		v.stack[v.sp] = nil
		v.sp--
		v.mu.Unlock()
	})()

	return v.stack[v.sp], nil
}

func (v *ViewStack) Peek() ViewFunc {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.stack[v.sp]
}
