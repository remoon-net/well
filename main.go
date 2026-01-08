package main

import "remoon.net/well/cmd"

func main() {
	cmd.Main("")
	<-cmd.ExitCh
}
