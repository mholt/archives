package archives

import (
	"bytes"
    "fmt"
    "strings"
	"io"
	"testing"
)

func TestXzParallelReader(t *testing.T) {
	data := []byte("hello world, this is a test for parallel xz decompression. it needs to be long enough or we can just use small data.")

	buf := new(bytes.Buffer)
	wc, err := Xz{}.OpenWriter(buf)
	if err != nil {
		t.Fatalf("failed to open writer: %v", err)
	}
	_, err = wc.Write(data)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	err = wc.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	compressed := buf.Bytes()

	// bytes.Reader реализует как io.ReaderAt, так и io.Seeker
	r := bytes.NewReader(compressed)

	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer rc.Close()

	decompressed, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read decompressed: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Errorf("decompressed data mismatch. got %q, want %q", decompressed, data)
	}
}

func TestXzParallelReaderDetection(t *testing.T) {
	data := []byte("check if parallel reader is actually used")
	buf := new(bytes.Buffer)
	wc, _ := Xz{}.OpenWriter(buf)
	wc.Write(data)
	wc.Close()

	r := bytes.NewReader(buf.Bytes())
	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer rc.Close()

	// В форке github.com/unxed/xz тип возвращаемого ридера будет xz.ParallelReader
	typeName := fmt.Sprintf("%T", rc)
	if !strings.Contains(typeName, "ParallelReader") {
		t.Errorf("expected parallel reader, got %s", typeName)
	}
}

// faultyReader имитирует поток, который ломается при попытке Seek
type faultyReader struct {
	*bytes.Reader
}

func (f faultyReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 0, io.ErrUnexpectedEOF // Имитируем ошибку при определении размера
	}
	return f.Reader.Seek(offset, whence)
}

func TestXzSeekErrorFallback(t *testing.T) {
	data := []byte("fallback data for seek error")
	buf := new(bytes.Buffer)
	wc, _ := Xz{}.OpenWriter(buf)
	wc.Write(data)
	wc.Close()

	// Оборачиваем в faultyReader, который упадет в streamSizeBySeeking
	r := faultyReader{bytes.NewReader(buf.Bytes())}

	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("OpenReader should not fail even if seek fails: %v", err)
	}
	defer rc.Close()

	decompressed, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Reading should succeed via sequential fallback: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Errorf("mismatch: got %q, want %q", decompressed, data)
	}
}

func TestXzEmptyStream(t *testing.T) {
	r := bytes.NewReader([]byte{})
	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		// NewReader может вернуть ошибку сразу для пустого потока, это нормально
		return
	}
	defer rc.Close()
	io.ReadAll(rc)
}

func TestXzInvalidStream(t *testing.T) {
	r := bytes.NewReader([]byte("THIS IS NOT XZ DATA"))
	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		return // Ошибка при открытии — это успех для битых данных
	}
	defer rc.Close()
	_, err = io.ReadAll(rc)
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

func TestXzParallelReaderPartialRead(t *testing.T) {
	data := bytes.Repeat([]byte("large data block "), 1000)
	buf := new(bytes.Buffer)
	wc, _ := Xz{}.OpenWriter(buf)
	wc.Write(data)
	wc.Close()

	r := bytes.NewReader(buf.Bytes())
	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// Читаем только верхушку
	smallBuf := make([]byte, 10)
	_, err = io.ReadFull(rc, smallBuf)
	if err != nil {
		t.Fatalf("partial read failed: %v", err)
	}

	// Закрываем ридер до того, как всё вычитали (проверка на утечки/паники)
	err = rc.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestXzSequentialReader(t *testing.T) {
	data := []byte("sequential test data")

	buf := new(bytes.Buffer)
	wc, err := Xz{}.OpenWriter(buf)
	if err != nil {
		t.Fatalf("failed to open writer: %v", err)
	}
	_, err = wc.Write(data)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	err = wc.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	compressed := buf.Bytes()

	// Использование bytes.Buffer (только io.Reader), чтобы спровоцировать fallback
	r := bytes.NewBuffer(compressed)

	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer rc.Close()

	decompressed, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read decompressed: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Errorf("decompressed data mismatch. got %q, want %q", decompressed, data)
	}
}

func TestXzParallelReaderWithOffset(t *testing.T) {
	data := []byte("hello world, testing parallel reader with offset. The stream doesn't start at 0.")

	buf := new(bytes.Buffer)
	wc, err := Xz{}.OpenWriter(buf)
	if err != nil {
		t.Fatalf("failed to open writer: %v", err)
	}
	_, err = wc.Write(data)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	err = wc.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	compressed := buf.Bytes()

	// Имитация чтения из центра какого-либо другого файла (например, ZIP или TAR)
	padding := []byte("GARBAGE DATA PADDING ")
	fullData := append(padding, compressed...)

	r := bytes.NewReader(fullData)

	// Пропускаем мусорные данные, чтобы позиция в потоке соответствовала началу XZ
	_, err = r.Seek(int64(len(padding)), io.SeekStart)
	if err != nil {
		t.Fatalf("failed to seek: %v", err)
	}

	// Инициализация должна определить, что мы смещены (currentOffset > 0)
	// и корректно инкапсулировать Reader через io.NewSectionReader
	rc, err := Xz{}.OpenReader(r)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer rc.Close()

	decompressed, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read decompressed: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Errorf("decompressed data mismatch. got %q, want %q", decompressed, data)
	}
}