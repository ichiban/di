package di

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type foo struct {
	err    error
	closed bool
}

func newFoo() (*foo, error) {
	return &foo{}, nil
}

func (f *foo) Close() error {
	f.closed = true
	return f.err
}

type bar struct {
	Foo *foo
}

func newBar(foo *foo) *bar {
	return &bar{
		Foo: foo,
	}
}

type qux struct {
	err    error
	closed bool
}

func newQux() (*qux, error) {
	return nil, errors.New("fail")
}

func (q *qux) Close() error {
	q.closed = true
	return q.err
}

func TestNew(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert := assert.New(t)

		c, err := New()
		assert.NoError(err)
		assert.Len(c.providers, 0)
		assert.Len(c.instances, 0)
	})

	t.Run("single", func(t *testing.T) {
		assert := assert.New(t)

		c, err := New(newFoo)
		assert.NoError(err)
		assert.Len(c.providers, 1)
		assert.Len(c.instances, 0)
	})

	t.Run("multiple", func(t *testing.T) {
		assert := assert.New(t)

		c, err := New(newFoo, newBar)
		assert.NoError(err)
		assert.Len(c.providers, 2)
		assert.Len(c.instances, 0)
	})
}

func TestContainer_Inject(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert := assert.New(t)

		c := Container{
			providers: map[reflect.Type]interface{}{},
		}

		var f *foo
		assert.Error(c.Inject(&f))
	})

	t.Run("single", func(t *testing.T) {
		assert := assert.New(t)

		c := Container{
			providers: map[reflect.Type]interface{}{
				reflect.TypeOf(&foo{}): newFoo,
			},
			instances: map[reflect.Type]interface{}{},
		}

		var f *foo
		assert.NoError(c.Inject(&f))
		assert.NotNil(f)
		_, ok := c.instances[reflect.TypeOf(&foo{})]
		assert.True(ok)
	})

	t.Run("chained", func(t *testing.T) {
		assert := assert.New(t)

		c := Container{
			providers: map[reflect.Type]interface{}{
				reflect.TypeOf(&foo{}): newFoo,
				reflect.TypeOf(&bar{}): newBar,
			},
			instances: map[reflect.Type]interface{}{},
		}

		var b *bar
		assert.NoError(c.Inject(&b))
		assert.NotNil(b)
		_, ok := c.instances[reflect.TypeOf(&foo{})]
		assert.True(ok)
		_, ok = c.instances[reflect.TypeOf(&bar{})]
		assert.True(ok)
	})

	t.Run("failure", func(t *testing.T) {
		assert := assert.New(t)

		c := Container{
			providers: map[reflect.Type]interface{}{
				reflect.TypeOf(&qux{}): newQux,
			},
			instances: map[reflect.Type]interface{}{},
		}

		var q *qux
		assert.Error(c.Inject(&q))
	})
}

func TestContainer_Close(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		f := foo{}
		b := bar{}

		c := Container{
			instances: map[reflect.Type]interface{}{
				reflect.TypeOf(&foo{}): &f,
				reflect.TypeOf(&bar{}): &b,
			},
		}

		assert.NoError(c.Close())
		assert.True(f.closed)
	})

	t.Run("failure", func(t *testing.T) {
		assert := assert.New(t)

		f := foo{
			err: errors.New("test"),
		}
		b := bar{}

		c := Container{
			instances: map[reflect.Type]interface{}{
				reflect.TypeOf(&foo{}): &f,
				reflect.TypeOf(&bar{}): &b,
			},
		}

		err := c.Close()
		assert.Error(err)
		assert.Equal("[test]", err.Error())
	})

	t.Run("multiple failures", func(t *testing.T) {
		assert := assert.New(t)

		f := foo{
			err: errors.New("1"),
		}

		q := qux{
			err: errors.New("2"),
		}

		c := Container{
			instances: map[reflect.Type]interface{}{
				reflect.TypeOf(&foo{}): &f,
				reflect.TypeOf(&qux{}): &q,
			},
		}

		err := c.Close()
		assert.Error(err)
		assert.Contains([]string{"[1 2]", "[2 1]"}, err.Error())
		assert.True(f.closed)
		assert.True(q.closed)
	})
}
