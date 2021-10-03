module go-m4a-wav-decode

replace github.com/alfg/mp4 => ../mp4

go 1.17

require (
	github.com/alfg/mp4 v0.0.0-20210728035756-55ea58c08aeb
	github.com/cryptix/wav v0.0.0-20180415113528-8bdace674401
	github.com/winlinvip/go-fdkaac v0.0.0-20180716140705-2654f5a0cc2e
)

require github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
