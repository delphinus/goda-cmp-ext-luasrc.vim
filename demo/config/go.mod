module goda-cmp-ext-luasrc-demo

go 1.26.0

require (
	github.com/Shougo/goda-cmp-ext v0.1.1
	github.com/Shougo/goda-cmp.vim v0.4.1
	github.com/delphinus/goda-cmp-ext-luasrc.vim v0.0.1
)

require (
	github.com/Shougo/goda-cmp-ui-pum.vim v0.1.0 // indirect
	github.com/Shougo/goda.vim v0.8.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

// Always the checkout this demo is in.
replace github.com/delphinus/goda-cmp-ext-luasrc.vim => ../..
