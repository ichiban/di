package di

import (
	"fmt"
	"io"
	"reflect"

	"golang.org/x/xerrors"
)

type Container struct {
	providers map[string]interface{}
	instances map[string]interface{}
}

func New(providers ...interface{}) (*Container, error) {
	c := Container{
		providers: make(map[string]interface{}, len(providers)),
		instances: make(map[string]interface{}, len(providers)),
	}

	for _, p := range providers {
		if err := c.provide(p); err != nil {
			return nil, xerrors.Errorf("failed to provide: %w", err)
		}
	}

	return &c, nil
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func (c *Container) provide(f interface{}) error {
	t := reflect.TypeOf(f)
	if t.Kind() != reflect.Func {
		return fmt.Errorf("not a function: %s", t)
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
	c.providers[o.String()] = f

	return nil
}

func (c *Container) Inject(o interface{}) error {
	v := reflect.ValueOf(o)
	t := v.Type()
	if t.Kind() != reflect.Ptr {
		return fmt.Errorf("not a pointer: %s", t)
	}

	return c.inject(v.Elem())
}

func (c *Container) inject(v reflect.Value) error {
	t := v.Type()

	o, err := c.instance(t.String())
	if err != nil {
		return err
	}

	if !v.CanSet() {
		return fmt.Errorf("cannot set: %s", t)
	}

	v.Set(reflect.ValueOf(o))

	return nil
}

func (c *Container) instance(name string) (interface{}, error) {
	i, ok := c.instances[name]
	if ok {
		return i, nil
	}

	p, ok := c.providers[name]
	if !ok {
		return nil, fmt.Errorf("not provided: %s", name)
	}

	f := reflect.ValueOf(p)
	t := f.Type()

	args := make([]reflect.Value, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		a, err := c.instance(t.In(i).String())
		if err != nil {
			return nil, err
		}
		args[i] = reflect.ValueOf(a)
	}

	var o interface{}
	var e error

	ret := f.Call(args)
	switch len(ret) {
	case 1:
		o = ret[0].Interface()
	case 2:
		o = ret[0].Interface()
		if i := ret[1].Interface(); i != nil {
			e = i.(error)
		}
	default:
		e = fmt.Errorf("invalid provider: %s", name)
	}

	c.instances[name] = o

	return o, e
}

func (c *Container) Close() error {
	var e multiError
	for _, i := range c.instances {
		c, ok := i.(io.Closer)
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

type multiError []error

func (m multiError) Error() string {
	return fmt.Sprintf("%v", []error(m))
}
