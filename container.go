package di

import (
	"fmt"
	"io"
	"reflect"

	"golang.org/x/xerrors"
)

type Container struct {
	providers map[reflect.Type]interface{}
	instances map[reflect.Type]reflect.Value
}

func New(providers ...interface{}) (*Container, error) {
	c := Container{
		providers: make(map[reflect.Type]interface{}, len(providers)),
		instances: make(map[reflect.Type]reflect.Value, len(providers)),
	}

	for _, p := range providers {
		if err := c.provide(p); err != nil {
			return nil, xerrors.Errorf("failed to provide: %w", err)
		}
	}

	return &c, nil
}

func MustNew(providers ...interface{}) *Container {
	c, err := New(providers...)
	if err != nil {
		panic(err)
	}
	return c
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func (c *Container) provide(provider interface{}) error {
	t := reflect.TypeOf(provider)
	if t.Kind() != reflect.Func {
		return fmt.Errorf("not a provider function: %s", t)
	}

	n := t.NumOut()
	if n == 0 || n > 2 {
		return fmt.Errorf("not a provider function: %s", t)
	}

	if n == 2 {
		e := t.Out(1)
		if e.Kind() != reflect.Interface || !e.Implements(errorInterface) {
			return fmt.Errorf("not a provider function with error: %s, %s", t, e)
		}
	}

	o := t.Out(0)
	if _, ok := c.providers[o]; ok {
		return fmt.Errorf("duplicated provider: %s", o)
	}
	c.providers[o] = provider

	return nil
}

func (c *Container) Consume(consumer interface{}) error {
	f := reflect.ValueOf(consumer)

	t := f.Type()
	if t.Kind() != reflect.Func {
		return xerrors.Errorf("not a consumer function: %s", t)
	}
	if t.NumOut() != 0 {
		return xerrors.Errorf("not a consumer function with 0 results: %s", t)
	}

	args := make([]reflect.Value, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		a, err := c.instance(t.In(i))
		if err != nil {
			return err
		}
		args[i] = a
	}

	f.Call(args)
	return nil
}

func (c *Container) MustConsume(consumer interface{}) {
	if err := c.Consume(consumer); err != nil {
		panic(err)
	}
}

func (c *Container) instance(ty reflect.Type) (reflect.Value, error) {
	i, ok := c.instances[ty]
	if ok {
		return i, nil
	}

	p, ok := c.providers[ty]
	if !ok {
		return reflect.Value{}, fmt.Errorf("not provided: %s", ty)
	}

	f := reflect.ValueOf(p)
	t := f.Type()

	args := make([]reflect.Value, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		a, err := c.instance(t.In(i))
		if err != nil {
			return reflect.Value{}, err
		}
		args[i] = a
	}

	var o reflect.Value
	var e error

	ret := f.Call(args)
	o = ret[0]

	if len(ret) == 2 {
		if i := ret[1].Interface(); i != nil {
			e = i.(error)
		}
	}

	c.instances[ty] = o

	return o, e
}

func (c *Container) Close() error {
	var e multiError
	for _, i := range c.instances {
		c, ok := i.Interface().(io.Closer)
		if !ok {
			continue
		}

		if err := c.Close(); err != nil {
			e = append(e, err)
		}
	}

	if len(e) != 0 {
		return e
	}

	return nil
}

func (c *Container) MustClose() {
	if err := c.Close(); err != nil {
		panic(err)
	}
}

type multiError []error

func (m multiError) Error() string {
	return fmt.Sprintf("%v", []error(m))
}
