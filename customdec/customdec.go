package customdec

import (
	"github.com/vbauerster/mpb/v8/decor"
)

type MarqueeTextDecor struct {
	CurrentIndex int
	String       string
	Size         int
}

func (c *MarqueeTextDecor) GetString() string {
	if c.CurrentIndex > len(c.String)-1 {
		c.CurrentIndex = 0
	}
	rstr := c.String[c.CurrentIndex:]
	if len(rstr) > c.Size {
		rstr = string(rstr[:c.Size])
	}
	if len(rstr) < c.Size {
		rstr += " " + c.String[:c.Size-len(rstr)]
	}
	return rstr
}

func (c *MarqueeTextDecor) Incr() {
	c.CurrentIndex += 1
}

func (c *MarqueeTextDecor) MarqueeText() decor.Decorator {
	return decor.Any(func(s decor.Statistics) string {
		if s.Completed {
			return c.String[:c.Size]
		}
		st := c.GetString()
		c.Incr()
		return st
	})
}

func NewChangeNameDecor(str string, size int) *MarqueeTextDecor {
	return &MarqueeTextDecor{CurrentIndex: 0, Size: size, String: str}
}
