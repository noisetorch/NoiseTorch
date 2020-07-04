//+build amd64,go1.10

#include "textflag.h"

GLOBL drawFillOver_SIMD_shufflemap<>(SB), (NOPTR+RODATA), $4
DATA drawFillOver_SIMD_shufflemap<>+0x00(SB)/4, $0x0d090501

TEXT ·drawFillOver_SIMD_internal(SB),0,$0-60
	// base+0(FP)
	// i0+8(FP)
	// i1+16(FP)
	// stride+24(FP)
	// n+32(FP)
	// adivm+40(FP)
	// sr+44(FP)
	// sg+48(FP)
	// sb+52(FP)
	// sa+56(FP)
	
	// DX row index
	// CX column index
	// AX pointer to current pixel
	// R14 i0
	// R15 i1
	
	// X0 zeroed register
	// X1 current pixel
	// X3 source pixel
	// X4 is the shuffle map to do the >> 8 and pack everything back into a single 32bit value
	
	MOVSS drawFillOver_SIMD_shufflemap<>(SB), X4
	
	PXOR X0, X0
	MOVQ i0+8(FP), R14
	MOVQ i1+16(FP), R15
	
	// load adivm to X2, fill all uint32s with it
	MOVSS advim+40(FP), X2
	VBROADCASTSS X2, X2
	
	// load source pixel to X3
	VMOVDQU sr+44(FP), X3
	
	MOVQ $0, DX
row_loop:
	CMPQ DX, n+32(FP)
	JGE row_loop_end
	
	MOVQ R14, CX
	MOVQ base+0(FP), AX
	LEAQ (AX)(CX*1), AX
column_loop:
	CMPQ CX, R15
	JGE column_loop_end
	
	// load current pixel to X1, unpack twice to get uint32s
	MOVSS (AX), X1
	PUNPCKLBW X0, X1
	VPUNPCKLWD X0, X1, X1
	
	VPMULLD X2, X1, X1 // component * a/m
	VPADDD X3, X1, X1 // (component * a/m) + source_component
	
	VPSHUFB X4, X1, X1 // get the second byte of every 32bit word and pack it into the lowest word of X1
	MOVSS X1, (AX) // write back to memory
	
	ADDQ $4, CX
	ADDQ $4, AX
	JMP column_loop
	
column_loop_end:
	ADDQ stride+24(FP), R14
	ADDQ stride+24(FP), R15
	INCQ DX
	JMP row_loop
	
row_loop_end:
	
	RET

TEXT ·getCPUID1(SB),$0
	MOVQ $1, AX
	CPUID
	MOVD DX, ret+0(FP)
	MOVD CX, ret+4(FP)
	RET

TEXT ·getCPUID70(SB),$0
	MOVQ $7, AX
	MOVQ $0, CX
	CPUID
	MOVD BX, ret+0(FP)
	MOVD CX, ret+4(FP)
	RET
