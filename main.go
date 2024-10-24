package main

import (
	"bufio"   // Paket untuk membaca input dari pengguna
	"fmt"     // Paket untuk format dan mencetak teks
	"log"     // Paket untuk logging atau menampilkan pesan error
	"net"     // Paket untuk jaringan (UDP, TCP)
	"os"      // Paket untuk akses argumen dari command line dan sistem operasi
	"os/signal" // Paket untuk menangani sinyal sistem seperti Ctrl+C
	"strings" // Paket untuk manipulasi string
	"syscall" // Paket untuk menangani sinyal-sinyal sistem
)

const (
	PORT = ":8080" // Mendefinisikan port yang akan digunakan oleh server
)

var username string // Variabel global untuk menyimpan nama pengguna

// Fungsi untuk memulai server dan mendengarkan pesan
func startServer() {
	addr, err := net.ResolveUDPAddr("udp", PORT) // Menghubungkan ke alamat UDP dengan port yang telah ditentukan
	if err != nil {
		log.Fatal(err) // Jika terjadi error, tampilkan pesan error dan keluar
	}

	conn, err := net.ListenUDP("udp", addr) // Membuat listener UDP untuk menerima pesan
	if err != nil {
		log.Fatal(err) // Jika error, log error dan keluar
	}
	defer conn.Close() // Tutup koneksi saat fungsi selesai

	clients := make(map[string]string) // Peta untuk menyimpan klien berdasarkan alamat dan nama pengguna

	fmt.Println("Server started on port", PORT) // Menampilkan bahwa server telah dimulai

	buffer := make([]byte, 1024) // Membuat buffer untuk menyimpan pesan yang diterima

	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer) // Membaca pesan dari klien
		if err != nil {
			log.Fatal(err) // Jika terjadi error, tampilkan pesan error
		}

		message := strings.TrimSpace(string(buffer[:n])) // Menghapus spasi di awal/akhir pesan dan mengubah buffer menjadi string

		// Jika pesan adalah "JOIN", tambahkan pengguna ke peta klien
		if strings.HasPrefix(message, "JOIN:") {
			user := strings.TrimPrefix(message, "JOIN:") // Menghapus prefix "JOIN:"
			clients[clientAddr.String()] = user          // Menyimpan nama pengguna dengan alamat klien
			fmt.Printf("%s joined the chat!\n", user)    // Menampilkan bahwa pengguna bergabung

			// Kirim notifikasi ke semua klien bahwa pengguna baru bergabung
			for addr := range clients {
				if addr != clientAddr.String() { // Kecuali pengguna yang baru bergabung, kirim pesan ke klien lain
					conn.WriteToUDP([]byte(user+" has joined the chat."), &net.UDPAddr{IP: net.ParseIP(strings.Split(addr, ":")[0]), Port: addrPort(addr)})
				}
			}
			continue
		}

		// Jika pesan adalah "LEAVE", hapus pengguna dari peta klien
		if strings.HasPrefix(message, "LEAVE:") {
			user := strings.TrimPrefix(message, "LEAVE:") // Menghapus prefix "LEAVE:"
			fmt.Printf("%s left the chat.\n", user)       // Menampilkan bahwa pengguna keluar
			delete(clients, clientAddr.String())          // Menghapus pengguna dari daftar klien

			// Kirim notifikasi ke semua klien bahwa pengguna telah keluar
			for addr := range clients {
				conn.WriteToUDP([]byte(user+" has left the chat."), &net.UDPAddr{IP: net.ParseIP(strings.Split(addr, ":")[0]), Port: addrPort(addr)})
			}
			continue
		}

		// Jika bukan pesan "JOIN" atau "LEAVE", broadcast pesan ke semua klien
		if username, ok := clients[clientAddr.String()]; ok { // Mengecek apakah klien terdaftar
			for addr := range clients {
				if addr != clientAddr.String() { // Broadcast pesan ke semua klien kecuali pengirim
					conn.WriteToUDP([]byte("["+username+"]: "+message), &net.UDPAddr{IP: net.ParseIP(strings.Split(addr, ":")[0]), Port: addrPort(addr)})
				}
			}
		}
	}
}

// Fungsi pembantu untuk mengekstrak port dari string alamat
func addrPort(addr string) int {
	parts := strings.Split(addr, ":") // Memisahkan alamat berdasarkan ":"
	port := 0
	fmt.Sscanf(parts[1], "%d", &port) // Membaca bagian kedua sebagai port
	return port                       // Mengembalikan port
}

// Fungsi untuk memulai klien untuk mengirim dan menerima pesan
func startClient() {
	addr, err := net.ResolveUDPAddr("udp", PORT) // Menghubungkan ke server UDP pada alamat yang ditentukan
	if err != nil {
		log.Fatal(err) // Jika terjadi error, tampilkan pesan error
	}

	conn, err := net.DialUDP("udp", nil, addr) // Membuat koneksi ke server
	if err != nil {
		log.Fatal(err) // Jika terjadi error, tampilkan pesan error
	}
	defer conn.Close() // Tutup koneksi saat selesai

	// Kirim pesan JOIN dengan nama pengguna ke server
	joinMessage := "JOIN:" + username
	_, err = conn.Write([]byte(joinMessage)) // Mengirim pesan JOIN ke server
	if err != nil {
		log.Fatal(err) // Jika terjadi error, tampilkan pesan error
	}

	// Channel untuk menangkap sinyal sistem (Ctrl+C)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM) // Menangkap sinyal SIGINT (Ctrl+C) dan SIGTERM

	// Goroutine untuk menangani sinyal keluar
	go func() {
		<-signalChan // Menunggu sinyal
		leaveMessage := "LEAVE:" + username
		_, err = conn.Write([]byte(leaveMessage)) // Mengirim pesan LEAVE ke server sebelum keluar
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("\nExiting...")
		os.Exit(0) // Keluar dari program
	}()

	// Channel untuk sinkronisasi pesan
	messageChannel := make(chan string)

	// Goroutine untuk mendengarkan pesan dari server
	go func() {
		buffer := make([]byte, 1024) // Buffer untuk menyimpan pesan
		for {
			n, _, err := conn.ReadFromUDP(buffer) // Membaca pesan dari server
			if err != nil {
				log.Fatal(err)
			}
			messageChannel <- string(buffer[:n]) // Mengirim pesan yang diterima ke channel
		}
	}()

	// Goroutine untuk membaca input dari pengguna
	go func() {
		reader := bufio.NewReader(os.Stdin) // Membaca input dari konsol
		for {
			fmt.Print("You: ")
			text, _ := reader.ReadString('\n') // Membaca input sampai enter
			text = strings.TrimSpace(text)     // Menghapus spasi di awal/akhir pesan

			_, err := conn.Write([]byte(text)) // Mengirim pesan ke server
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	// Loop utama untuk menampilkan pesan yang diterima tanpa mengganggu input pengguna
	for {
		select {
		case message := <-messageChannel:
			// Menampilkan pesan yang diterima
			fmt.Printf("\r%s\nYou: ", message) // Menghapus prompt "You: " sebelum menampilkan pesan, lalu menampilkannya lagi
		}
	}
}

func main() {
	if len(os.Args) < 3 { // Memastikan argumen cukup (mode dan nama pengguna)
		fmt.Println("Usage: go run main.go [server|client] [username]")
		return
	}

	mode := os.Args[1]    // Argumen pertama adalah mode (server atau client)
	username = os.Args[2] // Argumen kedua adalah nama pengguna

	if mode == "server" {
		startServer() // Memulai server jika mode adalah server
	} else if mode == "client" {
		startClient() // Memulai client jika mode adalah client
	} else {
		fmt.Println("Invalid mode. Use 'server' or 'client'.")
	}
}
