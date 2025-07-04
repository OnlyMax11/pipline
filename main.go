package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const bufferCapacity = 10
const flushInterval = 20 * time.Second

// Создание интерфейса для буфера
type RingBuffer interface {
	Push(value int, checker Check) bool
	PopAll() []int
}

// создание структуры буфера
type RingBufferImpl struct {
	buffer []int
	size   int
	head   int
	tail   int
	count  int
	mu     sync.Mutex
}

// контсруктор буфера
func NewRingBuffer(size int) RingBuffer {
	return &RingBufferImpl{
		buffer: make([]int, size),
		size:   size,
		head:   0,
		tail:   0,
		count:  0,
	}
}

// интерфес для проверки входящих чисел
type Check interface {
	Filter(value int) bool
}

type Num struct {
	mu sync.Mutex
}

// функция проверки чисел
func (n *Num) Filter(value int) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if value > 0 && value%3 != 0 {
		return true
	} else {
		return false
	}
}

func (rb *RingBufferImpl) Push(value int, checker Check) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if !checker.Filter(value) {
		log(fmt.Sprintf("Значение %d не прошло проверку и не будет добавлено в буфер.\n", value))
		return false
	}

	if rb.count == rb.size {
		log("Буфер переполнен!")
		return false
	}

	rb.buffer[rb.tail] = value
	rb.tail = (rb.tail + 1) % rb.size
	rb.count++
	log(fmt.Sprintf("Значение %d добавлено в буфер.\n", value))
	return true
}

func (rb *RingBufferImpl) PopAll() []int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		log("Буфер пуст")
		return nil
	}

	result := make([]int, rb.count)
	for i := 0; i < rb.count; i++ {
		result[i] = rb.buffer[(rb.head+i)%rb.size]
	}
	rb.head = (rb.head + rb.count) % rb.size
	rb.count = 0
	log("Все элементы извлечены из буфера")
	return result
}

func dataSourse(out chan<- int) {
	scanner := bufio.NewScanner(os.Stdin)
	log("Стадия: Ввод данных из консоли:")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "exit" {
			log("Команда завершения ввода получена.")
			close(out)
			break
		}

		num, err := strconv.Atoi(line)
		if err != nil || num < -99999 || num > 99999 {
			log("Некорректный ввод. Введите целое число.")
			continue
		}

		out <- num
	}
	if err := scanner.Err(); err != nil {
		log(fmt.Sprintf("Ошибка чтения ввода: %v", err))
		close(out)
	}
}

func bufferStage(in <-chan int, out chan<- int, capacity int, interval time.Duration, checker Check) {
	var buffers []RingBuffer
	currentBuffer := NewRingBuffer(capacity)
	buffers = append(buffers, currentBuffer)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log("Стадия: Буферизация данных")

	// Флаг для завершения работы
	done := false

	go func() {
		defer close(out)

		for !done {
			select {
			case num, ok := <-in:
				if !ok {
					log("Канал входных данных закрыт")
					for _, buf := range buffers {
						data := buf.PopAll()
						log(fmt.Sprintf("Отправка остаточных данных из буфера: %v", data))
						for _, val := range data {
							out <- val
						}
					}
					done = true
					return
				}
				log(fmt.Sprintf("Получено число для буферизации: %d", num))
				if !currentBuffer.Push(num, checker) {
					fmt.Printf("Буфер переполнен, создается новый буфер")
					newBuffer := NewRingBuffer(capacity)
					newBuffer.Push(num, checker)
					buffers = append(buffers, newBuffer)
					currentBuffer = newBuffer
				}
			case <-ticker.C:
				data := currentBuffer.PopAll()
				if len(data) > 0 {
					log(fmt.Sprintf("Очистка буфера по таймеру: %v", data))
					for _, num := range data {
						out <- num
					}
				}
			}
		}
	}()
}

func dataConsumer(in <-chan int) {
	log("Стадия: Потребление данных")
	for num := range in {
		log(fmt.Sprintf("Получены данные: %d", num))
	}
	log("Потребление данных завершено")
}

// функция логирования действий
func log(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] %s\n", timestamp, message)
}

func main() {
	var wg sync.WaitGroup

	inputChannel := make(chan int, bufferCapacity)    // Канал для входных данных
	bufferedChannel := make(chan int, bufferCapacity) // Канал для буферизированных данных

	checker := &Num{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		dataSourse(inputChannel)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		bufferStage(inputChannel, bufferedChannel, bufferCapacity, flushInterval, checker)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		dataConsumer(bufferedChannel)
	}()

	log("Для завершения программы введите 'exit' в консоли.")
	wg.Wait()
	log("Программа завершена.")
}
