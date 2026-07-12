package main

import (
	"sync"
	"time"
)

type progressTracker struct {
	mu             sync.Mutex
	totalCount     int
	totalBytes     int64
	completedCount int
	completedBytes int64
	active         map[int64]int64
	currentName    string
	currentIndex   int
	currentBytes   int64
	currentTotal   int64
	fileStatus     string
	startedAt      time.Time
}

func newProgressTracker(totalCount int, totalBytes int64) *progressTracker {
	return &progressTracker{
		totalCount: totalCount,
		totalBytes: totalBytes,
		active:     make(map[int64]int64),
		startedAt:  time.Now(),
	}
}

func (p *progressTracker) setPhase(name string, index int, total int64, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentName = name
	p.currentIndex = index
	p.currentBytes = 0
	p.currentTotal = total
	p.fileStatus = status
}

func (p *progressTracker) markSkipped(file AlbumFile, index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completedCount++
	if file.SizeBytes > 0 {
		p.completedBytes += file.SizeBytes
	}
	delete(p.active, file.FileID)
	p.currentName = file.Name
	p.currentIndex = index
	p.currentBytes = file.SizeBytes
	p.currentTotal = file.SizeBytes
	p.fileStatus = "skipped"
}

func (p *progressTracker) markDone(file AlbumFile, index int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completedCount++
	if file.SizeBytes > 0 {
		p.completedBytes += file.SizeBytes
	}
	delete(p.active, file.FileID)
	p.currentName = file.Name
	p.currentIndex = index
	p.currentBytes = file.SizeBytes
	p.currentTotal = file.SizeBytes
	p.fileStatus = "done"
}

func (p *progressTracker) updateActive(file AlbumFile, index int, bytesDone, bytesTotal int64, status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if file.FileID > 0 {
		p.active[file.FileID] = bytesDone
	}
	p.currentName = file.Name
	p.currentIndex = index
	p.currentBytes = bytesDone
	p.currentTotal = bytesTotal
	p.fileStatus = status
}

func (p *progressTracker) activeBytesLocked() int64 {
	var active int64
	for _, bytes := range p.active {
		active += bytes
	}
	return active
}

func (p *progressTracker) snapshot(running bool, cancelled bool, errMsg string) DownloadProgress {
	p.mu.Lock()
	defer p.mu.Unlock()

	progress := DownloadProgress{
		Running:        running,
		Cancelled:      cancelled,
		CurrentName:    p.currentName,
		CurrentIndex:   p.currentIndex,
		CurrentBytes:   p.currentBytes,
		CurrentTotal:   p.currentTotal,
		CompletedCount: p.completedCount,
		TotalCount:     p.totalCount,
		TotalBytes:     p.totalBytes,
		CompletedBytes: p.completedBytes + p.activeBytesLocked(),
		FileStatus:     p.fileStatus,
		Error:          errMsg,
	}
	if !p.startedAt.IsZero() {
		progress.StartedAtMs = p.startedAt.UnixMilli()
	}
	return progress
}

func sumFileBytes(files []AlbumFile) int64 {
	var total int64
	for _, file := range files {
		if file.SizeBytes > 0 {
			total += file.SizeBytes
		}
	}
	return total
}

func formatETA(seconds float64) string {
	if seconds < 0 || seconds != seconds { // NaN
		return "--"
	}
	if seconds < 1 {
		return "<1s"
	}
	seconds = float64(int(seconds + 0.5))
	if seconds < 60 {
		return formatInt(int(seconds)) + "s"
	}
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	if minutes < 60 {
		if secs == 0 {
			return formatInt(minutes) + "m"
		}
		return formatInt(minutes) + "m " + formatInt(secs) + "s"
	}
	hours := minutes / 60
	minutes = minutes % 60
	if minutes == 0 {
		return formatInt(hours) + "h"
	}
	return formatInt(hours) + "h " + formatInt(minutes) + "m"
}

func formatInt(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return itoa(value)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func computeETASeconds(completedBytes, totalBytes int64, elapsed time.Duration) float64 {
	if completedBytes <= 0 || totalBytes <= completedBytes || elapsed <= 0 {
		return -1
	}
	rate := float64(completedBytes) / elapsed.Seconds()
	if rate <= 0 {
		return -1
	}
	return float64(totalBytes-completedBytes) / rate
}
