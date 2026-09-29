package paint

type Package struct {
	ID 			uint32
	CMD 		uint16
	PositionX 	int
	PositionY 	int
	Width 		int
	Height 		int
	Data     	[]byte
}
