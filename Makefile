# Makefile for Hatvan OS (Motorola 6809 / Hitachi 6309, Motorola 68000, and Zilog Z80)

REPO_DIR    := $(CURDIR)
BUILD_DIR   := $(REPO_DIR)/build

# Toolchains
GO          ?= go
PYTHON      ?= python3
MINIGOLF_DIR ?= $(shell cd ../minigolf && pwd)
MINIGOLF    ?= $(BUILD_DIR)/minigolf
ASM68K      ?= $(BUILD_DIR)/asm68k
ASM6809     ?= $(BUILD_DIR)/asm6809
ASMZ80      ?= $(BUILD_DIR)/asmz80
ASM1802     ?= $(BUILD_DIR)/asm1802
ASM6502     ?= $(BUILD_DIR)/asm6502
ASM8086     ?= $(BUILD_DIR)/asm8086
PAR4ASM     ?= $(BUILD_DIR)/par4asm
PAR4BUNDLE  ?= $(BUILD_DIR)/par4bundle
LWASM       ?= lwasm
OS9         ?= os9
SREC2DECB   := $(REPO_DIR)/scripts/srec2decb.py
BUILD_CMD_SH := $(REPO_DIR)/build-cmd.sh

# VM Targets
VM_6809     := $(BUILD_DIR)/gep9
VM_68K      := $(BUILD_DIR)/gepk
VM_Z80      := $(BUILD_DIR)/gepz
VM_1802     := $(BUILD_DIR)/gepc
VM_6502     := $(BUILD_DIR)/gep5
VM_8086     := $(BUILD_DIR)/gep0

# Kernel Targets
KERNEL_6809 := $(BUILD_DIR)/kernel_6809.decb
KERNEL_68K  := $(BUILD_DIR)/kernel_68k.srec
KERNEL_Z80  := $(BUILD_DIR)/kernel_z80.decb
KERNEL_1802 := $(BUILD_DIR)/kernel_1802.decb
KERNEL_6502 := $(BUILD_DIR)/kernel_6502.decb
KERNEL_8086 := $(BUILD_DIR)/kernel_8086.decb
RBF_6809    := $(BUILD_DIR)/rbf_6809.decb
RBF_68K     := $(BUILD_DIR)/rbf_68k.srec
RBF_Z80     := $(BUILD_DIR)/rbf_z80.decb
RBF_1802    := $(BUILD_DIR)/rbf_1802.decb
RBF_6502    := $(BUILD_DIR)/rbf_6502.decb
RBF_8086    := $(BUILD_DIR)/rbf_8086.decb
PROCFS_6809 := $(BUILD_DIR)/procfs_6809.decb
PROCFS_68K  := $(BUILD_DIR)/procfs_68k.srec
PROCFS_Z80  := $(BUILD_DIR)/procfs_z80.decb
PROCFS_1802 := $(BUILD_DIR)/procfs_1802.decb
PROCFS_6502 := $(BUILD_DIR)/procfs_6502.decb
PROCFS_8086 := $(BUILD_DIR)/procfs_8086.decb

# Command Targets
CMDS_6809   := $(BUILD_DIR)/echo.mod $(BUILD_DIR)/testcmd.mod $(BUILD_DIR)/testdecb.decb $(BUILD_DIR)/cat.mod $(BUILD_DIR)/sh.mod
CMDS_68K    := $(BUILD_DIR)/echok.decb $(BUILD_DIR)/dirk.decb $(BUILD_DIR)/dumpk.decb $(BUILD_DIR)/catk.decb $(BUILD_DIR)/shk.decb
CMDS_Z80    := $(BUILD_DIR)/shz.decb
GOLF_CMDS   := gecho gcat gdir gdump gsh gsh2 gtest gexpr gtrue gfalse gseektest clif
CMDS_GOLF_9 := $(patsubst %,$(REPO_DIR)/cmds/%.9.decb,$(GOLF_CMDS))
CMDS_GOLF_K := $(patsubst %,$(REPO_DIR)/cmds/%.k.decb,$(GOLF_CMDS))
CMDS_GOLF_Z := $(patsubst %,$(REPO_DIR)/cmds/%.z.decb,$(GOLF_CMDS))
CMDS_GOLF_5 := $(patsubst %,$(REPO_DIR)/cmds/%.5.decb,$(GOLF_CMDS))
CMDS_GOLF_0 := $(patsubst %,$(REPO_DIR)/cmds/%.0.decb,$(GOLF_CMDS))
GOLF_CMDS_C := gecho gcat gdir gdump gsh gsh2 gtrue gfalse gseektest clif
CMDS_GOLF_C := $(patsubst %,$(REPO_DIR)/cmds/%.c.decb,$(GOLF_CMDS_C))

# Disk Images
DISK_IMAGE  := $(BUILD_DIR)/disk0.dsk
TEST_DISK   := $(BUILD_DIR)/test.dsk

# Source Dependencies
COMMON_SRCS := $(wildcard $(REPO_DIR)/kernel/common/*.golf)
KLIB_SRCS   := $(wildcard $(REPO_DIR)/kernel/klib/*.golf)
M6809_SRCS  := $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm
M68K_SRCS   := $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(REPO_DIR)/kernel/m68k/trap_m68k.s
Z80_SRCS    := $(wildcard $(REPO_DIR)/kernel/z80/*.golf) $(REPO_DIR)/kernel/z80/cstart_z80.asm $(REPO_DIR)/kernel/z80/trap_z80.asm
C1802_SRCS  := $(wildcard $(REPO_DIR)/kernel/cosmac1802/*.golf) $(REPO_DIR)/kernel/cosmac1802/par4vmc.asm
M6502_SRCS  := $(wildcard $(REPO_DIR)/kernel/m6502/*.golf) $(REPO_DIR)/kernel/m6502/cstart_m6502.asm $(REPO_DIR)/kernel/m6502/trap_m6502.asm
I8086_SRCS  := $(wildcard $(REPO_DIR)/kernel/i8086/*.golf) $(REPO_DIR)/kernel/i8086/cstart_i8086.asm $(REPO_DIR)/kernel/i8086/trap_i8086.asm

.PHONY: all vms tools kernels cmds disk test test-interactive test-z80 test-6502 test-8086 test-quick clean

all: $(BUILD_DIR) vms kernels cmds disk

tools: $(MINIGOLF) $(ASM68K) $(ASM6809) $(ASMZ80) $(ASM1802) $(ASM6502) $(ASM8086) $(PAR4ASM) $(PAR4BUNDLE)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# --- Toolchain Binaries ---
$(MINIGOLF): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(MINIGOLF) .

$(ASM68K): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM68K) ./cmd/asm68k

$(ASM6809): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM6809) ./cmd/asm6809

$(ASMZ80): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASMZ80) ./cmd/asmz80

$(ASM1802): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM1802) ./cmd/asm1802

$(ASM6502): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM6502) ./cmd/asm6502

$(ASM8086): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM8086) ./cmd/asm8086

$(PAR4ASM): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(PAR4ASM) ./cmd/par4asm

$(PAR4BUNDLE): $(shell find $(REPO_DIR)/cmd/par4bundle -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $(PAR4BUNDLE) ./cmd/par4bundle

# --- Emulators ---
vms: $(VM_6809) $(VM_68K) $(VM_Z80) $(VM_1802) $(VM_6502) $(VM_8086)

$(VM_6809): $(shell find $(REPO_DIR)/cmd/gep9 $(REPO_DIR)/gep9 -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep9

$(VM_68K): $(shell find $(REPO_DIR)/cmd/gepk $(REPO_DIR)/gepk -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepk

$(VM_Z80): $(shell find $(REPO_DIR)/cmd/gepz $(REPO_DIR)/gepz -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepz

$(VM_1802): $(shell find $(REPO_DIR)/cmd/gepc $(REPO_DIR)/gepc -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepc

$(VM_6502): $(shell find $(REPO_DIR)/cmd/gep5 $(REPO_DIR)/gep5 -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep5

$(VM_8086): $(shell find $(REPO_DIR)/cmd/gep0 $(REPO_DIR)/gep0 -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep0

# --- Kernels & Drivers ---
kernels: $(KERNEL_6809) $(KERNEL_68K) $(KERNEL_Z80) $(KERNEL_1802) $(KERNEL_6502) $(KERNEL_8086) $(RBF_6809) $(RBF_68K) $(RBF_Z80) $(RBF_1802) $(RBF_6502) $(RBF_8086) $(PROCFS_6809) $(PROCFS_68K) $(PROCFS_Z80) $(PROCFS_1802) $(PROCFS_6502) $(PROCFS_8086)

# M6809 Kernel
$(BUILD_DIR)/kernel_6809.asm: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m M6809 \
		-I $(REPO_DIR)/kernel/m6809 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_6809.asm: $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(BUILD_DIR)/kernel_6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(BUILD_DIR)/kernel_6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm > $@

$(BUILD_DIR)/kernel_6809.decb: $(BUILD_DIR)/full_6809.asm $(ASM6809) | $(BUILD_DIR)
	$(ASM6809) -decb -l $(BUILD_DIR)/kernel_6809.list -o $@ $<

# M6809 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_6809.asm: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m M6809 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/m6809 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_6809.asm: $(REPO_DIR)/drivers/rbf/cstart_rbf_m6809.asm $(BUILD_DIR)/rbf_6809.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_m6809.asm $(BUILD_DIR)/rbf_6809.asm > $@

$(BUILD_DIR)/rbf_6809.decb: $(BUILD_DIR)/full_rbf_6809.asm $(ASM6809) | $(BUILD_DIR)
	$(ASM6809) -decb -l $(BUILD_DIR)/rbf_6809.list -o $@ $<

# M6809 PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_6809.asm: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m M6809 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/m6809 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_procfs_6809.asm: $(REPO_DIR)/drivers/procfs/cstart_procfs_m6809.asm $(BUILD_DIR)/procfs_6809.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/procfs/cstart_procfs_m6809.asm $(BUILD_DIR)/procfs_6809.asm > $@

$(BUILD_DIR)/procfs_6809.decb: $(BUILD_DIR)/full_procfs_6809.asm $(ASM6809) | $(BUILD_DIR)
	$(ASM6809) -decb -l $(BUILD_DIR)/procfs_6809.list -o $@ $<

# M68K Kernel
$(BUILD_DIR)/kernel_68k.s: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=k \
		-I $(REPO_DIR)/kernel/m68k \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_68k.s: $(REPO_DIR)/kernel/m68k/trap_m68k.s $(BUILD_DIR)/kernel_68k.s | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/m68k/trap_m68k.s $(BUILD_DIR)/kernel_68k.s > $@

$(BUILD_DIR)/kernel_68k.srec: $(BUILD_DIR)/full_68k.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

# M68K RBF Driver (Task 1)
$(BUILD_DIR)/rbf_68k.s: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=k \
		-I $(REPO_DIR)/kernel/m68k \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_68k.s: $(REPO_DIR)/drivers/rbf/cstart_rbf_m68k.s $(BUILD_DIR)/rbf_68k.s | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_m68k.s $(BUILD_DIR)/rbf_68k.s > $@

$(BUILD_DIR)/rbf_68k.srec: $(BUILD_DIR)/full_rbf_68k.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

# M68K PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_68k.s: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=k \
		-I $(REPO_DIR)/kernel/m68k \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_procfs_68k.s: $(REPO_DIR)/drivers/procfs/cstart_procfs_m68k.s $(BUILD_DIR)/procfs_68k.s | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/procfs/cstart_procfs_m68k.s $(BUILD_DIR)/procfs_68k.s > $@

$(BUILD_DIR)/procfs_68k.srec: $(BUILD_DIR)/full_procfs_68k.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

# Z80 Kernel
$(BUILD_DIR)/kernel_z80.asm: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/z80/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=z80 \
		-I $(REPO_DIR)/kernel/z80 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_z80.asm: $(REPO_DIR)/kernel/z80/cstart_z80.asm $(BUILD_DIR)/kernel_z80.asm $(REPO_DIR)/kernel/z80/trap_z80.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/z80/cstart_z80.asm $(BUILD_DIR)/kernel_z80.asm $(REPO_DIR)/kernel/z80/trap_z80.asm > $@
	echo " end cstart" >> $@

$(BUILD_DIR)/kernel_z80.decb: $(BUILD_DIR)/full_z80.asm $(ASMZ80) | $(BUILD_DIR)
	$(ASMZ80) -l $(BUILD_DIR)/kernel_z80.list -o $@ $<

# Z80 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_z80.asm: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=z80 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/z80 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_z80.asm: $(REPO_DIR)/drivers/rbf/cstart_rbf_z80.asm $(BUILD_DIR)/rbf_z80.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_z80.asm $(BUILD_DIR)/rbf_z80.asm > $@

$(BUILD_DIR)/rbf_z80.decb: $(BUILD_DIR)/full_rbf_z80.asm $(ASMZ80) | $(BUILD_DIR)
	$(ASMZ80) -l $(BUILD_DIR)/rbf_z80.list -o $@ $<

# Z80 PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_z80.asm: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=z80 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/z80 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_procfs_z80.asm: $(REPO_DIR)/drivers/procfs/cstart_procfs_z80.asm $(BUILD_DIR)/procfs_z80.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/procfs/cstart_procfs_z80.asm $(BUILD_DIR)/procfs_z80.asm > $@

$(BUILD_DIR)/procfs_z80.decb: $(BUILD_DIR)/full_procfs_z80.asm $(ASMZ80) | $(BUILD_DIR)
	$(ASMZ80) -l $(BUILD_DIR)/procfs_z80.list -o $@ $<

# 1802 Par4 VM Runtime
$(BUILD_DIR)/par4vmc.bin: $(REPO_DIR)/kernel/cosmac1802/par4vmc.asm $(ASM1802) | $(BUILD_DIR)
	$(ASM1802) -format raw -l $(BUILD_DIR)/par4vmc.list -o $@ $<

# Z80 Userland Par4 VM Runtime
$(BUILD_DIR)/par4vmz.bin: $(REPO_DIR)/cmds/lib/par4vmz.asm $(ASMZ80) | $(BUILD_DIR)
	$(ASMZ80) -format raw -l $(BUILD_DIR)/par4vmz.list -o $@ $<

# 1802 Kernel (Task 0)
$(BUILD_DIR)/kernel_1802.p4s: $(COMMON_SRCS) $(KLIB_SRCS) $(C1802_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m par4 \
		-I $(MINIGOLF_DIR)/par4-lib \
		-I $(REPO_DIR)/kernel/cosmac1802 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/kernel_1802.p4p: $(BUILD_DIR)/kernel_1802.p4s $(PAR4ASM) | $(BUILD_DIR)
	$(PAR4ASM) -o $@ $<

$(BUILD_DIR)/kernel_1802.decb: $(BUILD_DIR)/kernel_1802.p4p $(BUILD_DIR)/par4vmc.bin $(BUILD_DIR)/kernel_1802.p4s $(PAR4BUNDLE) | $(BUILD_DIR)
	$(PAR4BUNDLE) -vm $(BUILD_DIR)/par4vmc.bin -p4p $< -p4s $(BUILD_DIR)/kernel_1802.p4s -entry 0x0000 -base 0x2000 -o $@


# 1802 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_1802.p4s: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(C1802_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m par4 \
		-I $(MINIGOLF_DIR)/par4-lib \
		-I $(REPO_DIR)/kernel/cosmac1802 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/rbf_1802.p4p: $(BUILD_DIR)/rbf_1802.p4s $(PAR4ASM) | $(BUILD_DIR)
	$(PAR4ASM) -o $@ $<

$(BUILD_DIR)/rbf_1802.decb: $(BUILD_DIR)/rbf_1802.p4p $(BUILD_DIR)/par4vmc.bin $(PAR4BUNDLE) | $(BUILD_DIR)
	$(PAR4BUNDLE) -vm $(BUILD_DIR)/par4vmc.bin -p4p $< -entry 0x0020 -base 0x2000 -o $@

# 1802 PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_1802.p4s: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(C1802_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m par4 \
		-I $(MINIGOLF_DIR)/par4-lib \
		-I $(REPO_DIR)/kernel/cosmac1802 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/procfs_1802.p4p: $(BUILD_DIR)/procfs_1802.p4s $(PAR4ASM) | $(BUILD_DIR)
	$(PAR4ASM) -o $@ $<

$(BUILD_DIR)/procfs_1802.decb: $(BUILD_DIR)/procfs_1802.p4p $(BUILD_DIR)/par4vmc.bin $(PAR4BUNDLE) | $(BUILD_DIR)
	$(PAR4BUNDLE) -vm $(BUILD_DIR)/par4vmc.bin -p4p $< -entry 0x0020 -base 0x2000 -o $@

# M6502 Kernel (Task 0)
$(BUILD_DIR)/kernel_6502.asm: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/m6502/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=6502 \
		-I $(REPO_DIR)/kernel/m6502 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_6502.asm: $(REPO_DIR)/kernel/m6502/cstart_m6502.asm $(BUILD_DIR)/kernel_6502.asm $(REPO_DIR)/kernel/m6502/trap_m6502.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/m6502/cstart_m6502.asm $(BUILD_DIR)/kernel_6502.asm $(REPO_DIR)/kernel/m6502/trap_m6502.asm > $@

$(BUILD_DIR)/kernel_6502.decb: $(BUILD_DIR)/full_6502.asm $(ASM6502) | $(BUILD_DIR)
	$(ASM6502) -l $(BUILD_DIR)/kernel_6502.list -o $@ $<

# M6502 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_6502.asm: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=6502 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/m6502 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_6502.asm: $(BUILD_DIR)/rbf_6502.asm $(REPO_DIR)/drivers/rbf/cstart_rbf_m6502.asm | $(BUILD_DIR)
	cat $(BUILD_DIR)/rbf_6502.asm $(REPO_DIR)/drivers/rbf/cstart_rbf_m6502.asm > $@

$(BUILD_DIR)/rbf_6502.decb: $(BUILD_DIR)/full_rbf_6502.asm $(ASM6502) | $(BUILD_DIR)
	$(ASM6502) -l $(BUILD_DIR)/rbf_6502.list -o $@ $<

# M6502 PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_6502.asm: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=6502 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/m6502 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_procfs_6502.asm: $(BUILD_DIR)/procfs_6502.asm $(REPO_DIR)/drivers/procfs/cstart_procfs_m6502.asm | $(BUILD_DIR)
	cat $(BUILD_DIR)/procfs_6502.asm $(REPO_DIR)/drivers/procfs/cstart_procfs_m6502.asm > $@

$(BUILD_DIR)/procfs_6502.decb: $(BUILD_DIR)/full_procfs_6502.asm $(ASM6502) | $(BUILD_DIR)
	$(ASM6502) -l $(BUILD_DIR)/procfs_6502.list -o $@ $<

# Intel 8086 Kernel (Task 0)
$(BUILD_DIR)/kernel_8086.asm: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/i8086/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=8086 \
		-global_var_offset 1024 \
		-I $(REPO_DIR)/kernel/i8086 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_8086.asm: $(REPO_DIR)/kernel/i8086/cstart_i8086.asm $(BUILD_DIR)/kernel_8086.asm $(REPO_DIR)/kernel/i8086/trap_i8086.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/i8086/cstart_i8086.asm $(BUILD_DIR)/kernel_8086.asm $(REPO_DIR)/kernel/i8086/trap_i8086.asm > $@

$(BUILD_DIR)/kernel_8086.decb: $(BUILD_DIR)/full_8086.asm $(ASM8086) | $(BUILD_DIR)
	$(ASM8086) -l $(BUILD_DIR)/kernel_8086.list -o $@ $<

# Intel 8086 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_8086.asm: $(REPO_DIR)/drivers/rbf/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=8086 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/i8086 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_8086.asm: $(REPO_DIR)/drivers/rbf/cstart_rbf_i8086.asm $(BUILD_DIR)/rbf_8086.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_i8086.asm $(BUILD_DIR)/rbf_8086.asm > $@
	echo "	END driver_start" >> $@

$(BUILD_DIR)/rbf_8086.decb: $(BUILD_DIR)/full_rbf_8086.asm $(ASM8086) | $(BUILD_DIR)
	$(ASM8086) -l $(BUILD_DIR)/rbf_8086.list -o $@ $<

# Intel 8086 PROCFS Driver (Task 2)
$(BUILD_DIR)/procfs_8086.asm: $(REPO_DIR)/drivers/procfs/main.golf $(COMMON_SRCS) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=8086 \
		-global_var_offset 512 \
		-I $(REPO_DIR)/kernel/i8086 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_procfs_8086.asm: $(REPO_DIR)/drivers/procfs/cstart_procfs_i8086.asm $(BUILD_DIR)/procfs_8086.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/procfs/cstart_procfs_i8086.asm $(BUILD_DIR)/procfs_8086.asm > $@
	echo "	END driver_start" >> $@

$(BUILD_DIR)/procfs_8086.decb: $(BUILD_DIR)/full_procfs_8086.asm $(ASM8086) | $(BUILD_DIR)
	$(ASM8086) -l $(BUILD_DIR)/procfs_8086.list -o $@ $<

# --- Userland Commands ---
cmds: $(CMDS_6809) $(CMDS_68K) $(CMDS_Z80) $(CMDS_GOLF_9) $(CMDS_GOLF_K) $(CMDS_GOLF_Z) $(CMDS_GOLF_C) $(CMDS_GOLF_5) $(CMDS_GOLF_0)

# MiniGolf Userland Commands
$(REPO_DIR)/cmds/%.9.decb $(REPO_DIR)/cmds/%.k.decb $(REPO_DIR)/cmds/%.z.decb $(REPO_DIR)/cmds/%.5.decb $(REPO_DIR)/cmds/%.0.decb: $(REPO_DIR)/cmds/%.golf $(REPO_DIR)/cmds/lib/sys.golf $(REPO_DIR)/cmds/lib/cstart_6809.asm $(REPO_DIR)/cmds/lib/cstart_68k.s $(REPO_DIR)/cmds/lib/cstart_z80.asm $(REPO_DIR)/cmds/lib/par4vmz.asm $(REPO_DIR)/cmds/lib/cstart_6502.asm $(REPO_DIR)/cmds/lib/cstart_8086.asm $(BUILD_CMD_SH) $(MINIGOLF) $(ASM68K) $(ASM6809) $(ASMZ80) $(ASM6502) $(ASM8086) $(PAR4ASM) $(PAR4BUNDLE) $(BUILD_DIR)/par4vmc.bin $(BUILD_DIR)/par4vmz.bin | $(BUILD_DIR)
	$(BUILD_CMD_SH) $<

$(CMDS_GOLF_C): $(REPO_DIR)/cmds/%.c.decb: $(REPO_DIR)/cmds/%.golf $(BUILD_CMD_SH) $(MINIGOLF) $(PAR4ASM) $(PAR4BUNDLE) $(BUILD_DIR)/par4vmc.bin $(BUILD_DIR)/par4vmz.bin | $(BUILD_DIR)
	$(BUILD_CMD_SH) $<

$(BUILD_DIR)/echo.mod: $(REPO_DIR)/cmds/echo.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=echo.list --map=echo.map -o echo.mod $<
	cp -f $(BUILD_DIR)/echo.mod.list $(BUILD_DIR)/echo.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/echo.mod.map $(BUILD_DIR)/echo.map 2>/dev/null || true

$(BUILD_DIR)/testcmd.mod: $(REPO_DIR)/cmds/testcmd.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=testcmd.list --map=testcmd.map -o testcmd.mod $<
	cp -f $(BUILD_DIR)/testcmd.mod.list $(BUILD_DIR)/testcmd.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/testcmd.mod.map $(BUILD_DIR)/testcmd.map 2>/dev/null || true

$(BUILD_DIR)/testdecb.decb: $(REPO_DIR)/cmds/testdecb.asm $(ASM6809) | $(BUILD_DIR)
	$(ASM6809) -decb -l $(BUILD_DIR)/testdecb.list -o $@ $<

$(BUILD_DIR)/echok.srec: $(REPO_DIR)/cmds/echok.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/echok.decb: $(BUILD_DIR)/echok.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/dirk.srec: $(REPO_DIR)/cmds/dirk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/dirk.decb: $(BUILD_DIR)/dirk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/dumpk.srec: $(REPO_DIR)/cmds/dumpk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/dumpk.decb: $(BUILD_DIR)/dumpk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/cat.mod: $(REPO_DIR)/cmds/cat.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=cat.list --map=cat.map -o cat.mod $<
	cp -f $(BUILD_DIR)/cat.mod.list $(BUILD_DIR)/cat.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/cat.mod.map $(BUILD_DIR)/cat.map 2>/dev/null || true

$(BUILD_DIR)/catk.srec: $(REPO_DIR)/cmds/catk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/catk.decb: $(BUILD_DIR)/catk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/sh.mod: $(REPO_DIR)/cmds/sh.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=sh.list --map=sh.map -o sh.mod $<
	cp -f $(BUILD_DIR)/sh.mod.list $(BUILD_DIR)/sh.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/sh.mod.map $(BUILD_DIR)/sh.map 2>/dev/null || true

$(BUILD_DIR)/shk.srec: $(REPO_DIR)/cmds/shk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/shk.decb: $(BUILD_DIR)/shk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/shz.decb: $(REPO_DIR)/cmds/shz.asm $(ASMZ80) | $(BUILD_DIR)
	$(ASMZ80) -l $(BUILD_DIR)/shz.list -o $@ $<

# --- OS-9 Disk Images ---
disk: $(DISK_IMAGE) $(TEST_DISK)

$(DISK_IMAGE): $(CMDS_6809) $(CMDS_68K) $(CMDS_Z80) $(CMDS_GOLF_9) $(CMDS_GOLF_K) $(CMDS_GOLF_Z) $(CMDS_GOLF_C) $(CMDS_GOLF_5) $(CMDS_GOLF_0) $(REPO_DIR)/cmds/os9-6809-level1.zip | $(BUILD_DIR)
	rm -f $@ $(TEST_DISK)
	$(OS9) format -e -n'HATVAN' -l'40000' $@
	$(OS9) makdir $@,Cmds9
	unzip -q -o $(REPO_DIR)/cmds/os9-6809-level1.zip -d $(BUILD_DIR)
	for f in $(BUILD_DIR)/os9-6809-level1/*; do $(OS9) copy -r "$$f" $@,Cmds9; done
	for f in $(BUILD_DIR)/os9-6809-level1/*; do $(OS9) attr -r -w -e -pr -pe $@,Cmds9/$$(basename $$f); done
	$(OS9) copy -r $(BUILD_DIR)/echo.mod $@,Cmds9/ECHO
	$(OS9) copy -r $(BUILD_DIR)/cat.mod $@,Cmds9/CAT
	$(OS9) copy -r $(BUILD_DIR)/sh.mod $@,Cmds9/SH
	$(OS9) copy -r $(BUILD_DIR)/testcmd.mod $@,Cmds9/TESTCMD
	$(OS9) copy -r $(BUILD_DIR)/testdecb.decb $@,Cmds9/TESTDECB
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.9.decb $@,Cmds9/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.9.decb $@,Cmds9/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.9.decb $@,Cmds9/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.9.decb $@,Cmds9/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.9.decb $@,Cmds9/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.9.decb $@,Cmds9/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.9.decb $@,Cmds9/GTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.9.decb $@,Cmds9/GEXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.9.decb $@,Cmds9/TEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.9.decb $@,Cmds9/EXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.9.decb $@,Cmds9/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.9.decb $@,Cmds9/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.9.decb $@,Cmds9/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.9.decb $@,Cmds9/CLIF
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/ECHO
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/CAT
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/SH
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/TESTCMD
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/TESTDECB
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GECHO
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GCAT
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GDIR
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GDUMP
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GSH
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GSH2
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GTEST
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GEXPR
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/TEST
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/EXPR
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GTRUE
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GFALSE
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/GSEEKTEST
	$(OS9) attr -r -w -e -pr -pe $@,Cmds9/CLIF
	$(OS9) makdir $@,CmdsK
	$(OS9) copy -r $(BUILD_DIR)/echok.decb $@,CmdsK/ECHO
	$(OS9) copy -r $(BUILD_DIR)/dirk.decb $@,CmdsK/DIR
	$(OS9) copy -r $(BUILD_DIR)/dumpk.decb $@,CmdsK/DUMP
	$(OS9) copy -r $(BUILD_DIR)/catk.decb $@,CmdsK/CAT
	$(OS9) copy -r $(BUILD_DIR)/shk.decb $@,CmdsK/SH
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.k.decb $@,CmdsK/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.k.decb $@,CmdsK/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.k.decb $@,CmdsK/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.k.decb $@,CmdsK/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.k.decb $@,CmdsK/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.k.decb $@,CmdsK/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.k.decb $@,CmdsK/GTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.k.decb $@,CmdsK/GEXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.k.decb $@,CmdsK/TEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.k.decb $@,CmdsK/EXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.k.decb $@,CmdsK/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.k.decb $@,CmdsK/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.k.decb $@,CmdsK/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.k.decb $@,CmdsK/CLIF
	$(OS9) makdir $@,CmdsZ
	$(OS9) copy -r $(BUILD_DIR)/shz.decb $@,CmdsZ/SH
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.z.decb $@,CmdsZ/ECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.z.decb $@,CmdsZ/CAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.z.decb $@,CmdsZ/DIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.z.decb $@,CmdsZ/DUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.z.decb $@,CmdsZ/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.z.decb $@,CmdsZ/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.z.decb $@,CmdsZ/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.z.decb $@,CmdsZ/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.z.decb $@,CmdsZ/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.z.decb $@,CmdsZ/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.z.decb $@,CmdsZ/GTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.z.decb $@,CmdsZ/GEXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.z.decb $@,CmdsZ/TEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.z.decb $@,CmdsZ/EXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.z.decb $@,CmdsZ/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.z.decb $@,CmdsZ/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.z.decb $@,CmdsZ/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.z.decb $@,CmdsZ/CLIF
	$(OS9) makdir $@,CmdsC
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.c.decb $@,CmdsC/SH
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.c.decb $@,CmdsC/ECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.c.decb $@,CmdsC/CAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.c.decb $@,CmdsC/DIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.c.decb $@,CmdsC/DUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.c.decb $@,CmdsC/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.c.decb $@,CmdsC/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.c.decb $@,CmdsC/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.c.decb $@,CmdsC/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.c.decb $@,CmdsC/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.c.decb $@,CmdsC/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.c.decb $@,CmdsC/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.c.decb $@,CmdsC/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.c.decb $@,CmdsC/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.c.decb $@,CmdsC/TRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.c.decb $@,CmdsC/FALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.c.decb $@,CmdsC/CLIF
	$(OS9) makdir $@,Cmds5
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.5.decb $@,Cmds5/SH
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.5.decb $@,Cmds5/ECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.5.decb $@,Cmds5/CAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.5.decb $@,Cmds5/DIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.5.decb $@,Cmds5/DUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.5.decb $@,Cmds5/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.5.decb $@,Cmds5/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.5.decb $@,Cmds5/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.5.decb $@,Cmds5/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.5.decb $@,Cmds5/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.5.decb $@,Cmds5/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.5.decb $@,Cmds5/GTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.5.decb $@,Cmds5/GEXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.5.decb $@,Cmds5/TEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.5.decb $@,Cmds5/EXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.5.decb $@,Cmds5/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.5.decb $@,Cmds5/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.5.decb $@,Cmds5/TRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.5.decb $@,Cmds5/FALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.5.decb $@,Cmds5/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.5.decb $@,Cmds5/CLIF
	$(OS9) makdir $@,Cmds0
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.0.decb $@,Cmds0/SH
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.0.decb $@,Cmds0/ECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.0.decb $@,Cmds0/CAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.0.decb $@,Cmds0/DIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.0.decb $@,Cmds0/DUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gecho.0.decb $@,Cmds0/GECHO
	$(OS9) copy -r $(REPO_DIR)/cmds/gcat.0.decb $@,Cmds0/GCAT
	$(OS9) copy -r $(REPO_DIR)/cmds/gdir.0.decb $@,Cmds0/GDIR
	$(OS9) copy -r $(REPO_DIR)/cmds/gdump.0.decb $@,Cmds0/GDUMP
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh.0.decb $@,Cmds0/GSH
	$(OS9) copy -r $(REPO_DIR)/cmds/gsh2.0.decb $@,Cmds0/GSH2
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.0.decb $@,Cmds0/GTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.0.decb $@,Cmds0/GEXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtest.0.decb $@,Cmds0/TEST
	$(OS9) copy -r $(REPO_DIR)/cmds/gexpr.0.decb $@,Cmds0/EXPR
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.0.decb $@,Cmds0/GTRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.0.decb $@,Cmds0/GFALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gtrue.0.decb $@,Cmds0/TRUE
	$(OS9) copy -r $(REPO_DIR)/cmds/gfalse.0.decb $@,Cmds0/FALSE
	$(OS9) copy -r $(REPO_DIR)/cmds/gseektest.0.decb $@,Cmds0/GSEEKTEST
	$(OS9) copy -r $(REPO_DIR)/cmds/clif.0.decb $@,Cmds0/CLIF
	$(OS9) copy -r $(REPO_DIR)/testdata/test_expr.sh $@,test_expr.sh
	$(OS9) copy -r $(REPO_DIR)/testdata/test_while.sh $@,test_while.sh
	$(OS9) copy -r $(REPO_DIR)/testdata/triangle.sh $@,triangle.sh
	cp -f $@ $(TEST_DISK)

$(TEST_DISK): $(DISK_IMAGE)

# --- Testing ---
test: $(BUILD_DIR) vms kernels cmds disk
	$(VM_6809) --disk0=$(DISK_IMAGE) --input="exit\n" $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) -input="exit\n" $(KERNEL_68K)
	$(VM_Z80) -disk0=$(DISK_IMAGE) -input="exit\n" $(KERNEL_Z80)
	$(VM_1802) -tick-hz=0 -disk0=$(DISK_IMAGE) -task1=$(RBF_1802) -task2=$(PROCFS_1802) -input="exit\n" $(KERNEL_1802)
	$(VM_6502) -disk0=$(DISK_IMAGE) -task1=$(RBF_6502) -task2=$(PROCFS_6502) -input="exit\n" $(KERNEL_6502)
	$(VM_8086) -disk0=$(DISK_IMAGE) -task1=$(RBF_8086) -task2=$(PROCFS_8086) -input="exit\n" $(KERNEL_8086)

test-6502: $(BUILD_DIR) vms kernels cmds disk
	$(GO) test -count=1 ./gep5/...
	$(VM_6502) -disk0=$(DISK_IMAGE) -task1=$(RBF_6502) -task2=$(PROCFS_6502) -input="dir\ndir e\nGECHO hello from 6502\nGCAT /proc/i\nGSEEKTEST\ngsh test_expr.sh\ngsh test_while.sh\ngsh triangle.sh\nexit\n" $(KERNEL_6502)

test-8086: $(BUILD_DIR) vms kernels cmds disk
	$(GO) test -count=1 ./gep0/...
	$(VM_8086) -disk0=$(DISK_IMAGE) -task1=$(RBF_8086) -task2=$(PROCFS_8086) -input="dir\ndir e\nGECHO hello from 8086\nGCAT /proc/i\nGSEEKTEST\ngsh test_expr.sh\ngsh test_while.sh\ngsh triangle.sh\ngsh2\necho hello from gsh2 on 8086 > /d0/redir0.txt\ncat /d0/redir0.txt\nexit\nexit\n" $(KERNEL_8086)

test-interactive: $(BUILD_DIR) vms kernels cmds disk
	$(VM_6809) --disk0=$(DISK_IMAGE) --input="help\npwd\npwx\nECHO hello from 6809 userspace\nECHO redirection works on 6809 > /d0/redir9.txt\nCAT /d0/redir9.txt\nGECHO hello from gecho 6809\nGCAT /d0/redir9.txt\nGDIR\nGDUMP /Cmds9/ECHO\nGSEEKTEST\nSH\nhelp\nECHO nested shell 6809\nCAT /nonexistent\nECHO background 6809 &\nCAT /proc/p\nexit\nCAT /nonexistent\nexit\n" $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) -input="help\npwd\npwx\nECHO hello from 68k userspace\nECHO redirection works on 68k > /d0/redirk.txt\nCAT /d0/redirk.txt\nGECHO hello from gecho 68k\nGCAT /d0/redirk.txt\nGDIR\nGDUMP /CmdsK/ECHO\nGSEEKTEST\nSH\nhelp\nECHO nested shell 68k\nCAT /nonexistent\nECHO background 68k &\nCAT /proc/p\nexit\nCAT /nonexistent\nexit\n" $(KERNEL_68K)
	$(VM_Z80) -disk0=$(DISK_IMAGE) -input="help\npwd\npwx\nECHO hello from z80 userspace\nECHO redirection works on z80 > /d0/redirz.txt\nCAT /d0/redirz.txt\nGECHO hello from gecho z80\nGCAT /d0/redirz.txt\nGDIR\nGDUMP /CmdsZ/ECHO\nGSEEKTEST\nSH\nhelp\nECHO nested shell z80\nCAT /nonexistent\nECHO background z80 &\nCAT /proc/p\nexit\nCAT /nonexistent\nexit\n" $(KERNEL_Z80)

test-z80: $(BUILD_DIR) vms kernels cmds disk
	(cd $(MINIGOLF_DIR) && $(GO) test -count=1 -run ".*z80.*|.*Z80.*" . && $(GO) test -count=1 ./cmd/asmz80)
	$(VM_Z80) -disk0=$(DISK_IMAGE) -input="exit\n" $(KERNEL_Z80)
	$(VM_Z80) -disk0=$(DISK_IMAGE) -input="help\npwd\npwx\nECHO hello from z80 userspace\nECHO redirection works on z80 > /d0/redirz.txt\nCAT /d0/redirz.txt\nGECHO hello from gecho z80\nGCAT /d0/redirz.txt\nGDIR\nGDUMP /CmdsZ/ECHO\nGSEEKTEST\nSH\nhelp\nECHO nested shell z80\nCAT /nonexistent\nECHO background z80 &\nCAT /proc/p\nexit\nCAT /nonexistent\nexit\n" $(KERNEL_Z80)

test-quick:
	@$(REPO_DIR)/scripts/test-quick.sh

# --- Clean ---
clean:
	@mkdir -p $(BUILD_DIR)
	find $(BUILD_DIR) -mindepth 1 -delete 2>/dev/null || rm -rf $(BUILD_DIR)/*
	rm -f $(REPO_DIR)/cmds/*.9.decb $(REPO_DIR)/cmds/*.k.decb $(REPO_DIR)/cmds/*.z.decb $(REPO_DIR)/cmds/*.c.decb $(REPO_DIR)/cmds/*.5.decb $(REPO_DIR)/cmds/*.0.decb
