package clock

import "time"

// Clock 抽象时间来源，便于测试注入与固定刻度。
type Clock interface {
	Now() time.Time
}

// System 真实系统时钟。
type System struct{}

func (System) Now() time.Time { return time.Now() }

// Fixed 固定时刻时钟（测试/演示）。
type Fixed struct {
	At time.Time
}

func (f Fixed) Now() time.Time { return f.At }

// Ticker 简单可替换 ticker 抽象。
type Ticker struct {
	C  <-chan time.Time
	Stop func()
}
