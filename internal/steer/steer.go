package steer

import (
	"errors"
	"os"
)

// Delivery wraps the inbox and FIFO paths used for soft steering.
type Delivery struct {
	InboxDir string
	FIFOPath string

	fifoReader *os.File
	fifoWriter *os.File
}

func (d *Delivery) Write(message string) error {
	return WriteInbox(d.InboxDir, message)
}

func (d *Delivery) Poll() ([]InboxMessage, error) {
	if !HasMessages(d.InboxDir) {
		return nil, nil
	}
	return ReadInbox(d.InboxDir)
}

func (d *Delivery) CreateChannels() error {
	if err := CreateInbox(d.InboxDir); err != nil {
		return err
	}
	if d.FIFOPath == "" {
		d.FIFOPath = Path(d.InboxDir)
	}
	if err := Create(d.FIFOPath); err != nil {
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		return err
	}
	reader, err := OpenReadNonblock(d.FIFOPath)
	if err != nil {
		_ = Remove(d.FIFOPath)
		return err
	}
	writer, err := OpenWriteNonblock(d.FIFOPath)
	if err != nil {
		_ = reader.Close()
		_ = Remove(d.FIFOPath)
		return err
	}
	d.fifoReader = reader
	d.fifoWriter = writer
	return nil
}

func (d *Delivery) Close() error {
	if d == nil {
		return nil
	}
	var errs []error
	if d.fifoReader != nil {
		if err := d.fifoReader.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	}
	if d.fifoWriter != nil {
		if err := d.fifoWriter.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			errs = append(errs, err)
		}
	}
	if d.FIFOPath != "" {
		if err := Remove(d.FIFOPath); err != nil && !errors.Is(err, ErrUnsupported) && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
