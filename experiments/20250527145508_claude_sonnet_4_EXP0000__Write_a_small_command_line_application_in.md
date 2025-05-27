package main

import (
    "fmt"
    "math/rand"
    "time"
)

const (
    reset  = "\033[0m"
    green  = "\033[32m"
    brown  = "\033[33m"
    dgreen = "\033[36m"
    lgreen = "\033[92m"
)

func main() {
    rand.Seed(time.Now().UnixNano())

    h := rand.Intn(8) + 15  // height 15-22
    w := rand.Intn(4) + 6   // crown width 6-9

    // Crown
    for i := 0; i < 4+rand.Intn(3); i++ {
        spaces := w - i
        if spaces < 0 {
            spaces = 0
        }

        // Left fronds
        fmt.Print(pad(spaces))
        if rand.Float32() < 0.7 {
            fmt.Print(frondColor() + leftFrond(i+2+rand.Intn(3)) + reset)
        }

        // Center
        fmt.Print(frondColor() + centerFrond(i+1) + reset)

        // Right fronds
        if rand.Float32() < 0.7 {
            fmt.Print(frondColor() + rightFrond(i+2+rand.Intn(3)) + reset)
        }
        fmt.Println()
    }

    // Trunk
    for i := 0; i < h-8; i++ {
        spaces := w + rand.Intn(2)
        fmt.Print(pad(spaces))
        fmt.Print(brown)

        if rand.Float32() < 0.3 {
            fmt.Print("||")
        } else if rand.Float32() < 0.6 {
            fmt.Print(")(")
        } else {
            fmt.Print("}{")
        }
        fmt.Println(reset)
    }

    // Base
    fmt.Print(pad(w))
    fmt.Print(brown + "^^^" + reset)
    fmt.Println()
}

func pad(n int) string {
    s := ""
    for i := 0; i < n; i++ {
        s += " "
    }
    return s
}

func frondColor() string {
    colors := []string{green, dgreen, lgreen}
    return colors[rand.Intn(len(colors))]
}

func leftFrond(n int) string {
    fronds := []string{"\\\\\\", "\\\\", "\\~\\", "\\~~", "~~~"}
    if n < len(fronds) {
        return fronds[n]
    }
    return fronds[len(fronds)-1]
}

func centerFrond(n int) string {
    fronds := []string{"|", "^", "/|\\", "/^\\", "/*\\", "/~*~\\"}
    if n < len(fronds) {
        return fronds[n]
    }
    return fronds[len(fronds)-1]
}

func rightFrond(n int) string {
    fronds := []string{"///", "//", "/~/", "~~/", "~~~"}
    if n < len(fronds) {
        return fronds[n]
    }
    return fronds[len(fronds)-1]
}
