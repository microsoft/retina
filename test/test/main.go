package main

import "fmt"

func main() {
    x := 0
    fmt.Println("Before the block, x:", x) // Output: Before the block, x: 0

    if true {
        x := 1 // This x shadows the outer x
        x++
        fmt.Println("Inside the block, x:", x) // Output: Inside the block, x: 2
    }

    fmt.Println("After the block, x:", x) // Output: After the block, x: 0
}