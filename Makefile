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
LWASM       ?= lwasm
OS9         ?= os9
SREC2DECB   := $(REPO_DIR)/scripts/srec2decb.py
BUILD_CMD_SH := $(REPO_DIR)/build-cmd.sh

# VM Targets
VM_6809     := $(BUILD_DIR)/gep9
VM_68K      := $(BUILD_DIR)/gepk
VM_Z80      := $(BUILD_DIR)/gepz
VM_1802     := $(BUILD_DIR)/gepc

# Kernel Targets
KERNEL_6809 := $(BUILD_DIR)/kernel_6809.decb
KERNEL_68K  := $(BUILD_DIR)/kernel_68k.srec
KERNEL_Z80  := $(BUILD_DIR)/kernel_z80.decb
RBF_6809    := $(BUILD_DIR)/rbf_6809.decb
RBF_68K     := $(BUILD_DIR)/rbf_68k.srec
RBF_Z80     := $(BUILD_DIR)/rbf_z80.decb
PROCFS_6809 := $(BUILD_DIR)/procfs_6809.decb
PROCFS_68K  := $(BUILD_DIR)/procfs_68k.srec
PROCFS_Z80  := $(BUILD_DIR)/procfs_z80.decb

# Command Targets
CMDS_6809   := $(BUILD_DIR)/echo.mod $(BUILD_DIR)/testcmd.mod $(BUILD_DIR)/testdecb.decb $(BUILD_DIR)/cat.mod $(BUILD_DIR)/sh.mod
CMDS_68K    := $(BUILD_DIR)/echok.decb $(BUILD_DIR)/dirk.decb $(BUILD_DIR)/dumpk.decb $(BUILD_DIR)/catk.decb $(BUILD_DIR)/shk.decb
CMDS_Z80    := $(BUILD_DIR)/shz.decb
GOLF_CMDS   := gecho gcat gdir gdump gsh gsh2 gtest gexpr gtrue gfalse gseektest
CMDS_GOLF_9 := $(patsubst %,$(REPO_DIR)/cmds/%.9.decb,$(GOLF_CMDS))
CMDS_GOLF_K := $(patsubst %,$(REPO_DIR)/cmds/%.k.decb,$(GOLF_CMDS))
CMDS_GOLF_Z := $(patsubst %,$(REPO_DIR)/cmds/%.z.decb,$(GOLF_CMDS))

# Disk Images
DISK_IMAGE  := $(BUILD_DIR)/disk0.dsk
TEST_DISK   := $(BUILD_DIR)/test.dsk

# Source Dependencies
COMMON_SRCS := $(wildcard $(REPO_DIR)/kernel/common/*.golf)
KLIB_SRCS   := $(wildcard $(REPO_DIR)/kernel/klib/*.golf)
M6809_SRCS  := $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm
M68K_SRCS   := $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(REPO_DIR)/kernel/m68k/trap_m68k.s
Z80_SRCS    := $(wildcard $(REPO_DIR)/kernel/z80/*.golf) $(REPO_DIR)/kernel/z80/cstart_z80.asm $(REPO_DIR)/kernel/z80/trap_z80.asm

.PHONY: all vms tools kernels cmds disk test test-interactive test-z80 test-quick clean

all: $(BUILD_DIR) vms kernels cmds disk

tools: $(MINIGOLF) $(ASM68K) $(ASM6809) $(ASMZ80) $(ASM1802)

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

# --- Emulators ---
vms: $(VM_6809) $(VM_68K) $(VM_Z80) $(VM_1802)

$(VM_6809): $(shell find $(REPO_DIR)/cmd/gep9 $(REPO_DIR)/gep9 -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep9

$(VM_68K): $(shell find $(REPO_DIR)/cmd/gepk $(REPO_DIR)/gepk -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepk

$(VM_Z80): $(shell find $(REPO_DIR)/cmd/gepz $(REPO_DIR)/gepz -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepz

$(VM_1802): $(shell find $(REPO_DIR)/cmd/gepc $(REPO_DIR)/gepc -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepc

# --- Kernels & Drivers ---
kernels: $(KERNEL_6809) $(KERNEL_68K) $(KERNEL_Z80) $(RBF_6809) $(RBF_68K) $(RBF_Z80) $(PROCFS_6809) $(PROCFS_68K) $(PROCFS_Z80)

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

# --- Userland Commands ---
cmds: $(CMDS_6809) $(CMDS_68K) $(CMDS_Z80) $(CMDS_GOLF_9) $(CMDS_GOLF_K) $(CMDS_GOLF_Z)

# MiniGolf Userland Commands
$(REPO_DIR)/cmds/%.9.decb $(REPO_DIR)/cmds/%.k.decb $(REPO_DIR)/cmds/%.z.decb: $(REPO_DIR)/cmds/%.golf $(REPO_DIR)/cmds/lib/sys.golf $(REPO_DIR)/cmds/lib/cstart_6809.asm $(REPO_DIR)/cmds/lib/cstart_68k.s $(REPO_DIR)/cmds/lib/cstart_z80.asm $(BUILD_CMD_SH) $(MINIGOLF) $(ASM68K) $(ASM6809) $(ASMZ80) | $(BUILD_DIR)
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

$(DISK_IMAGE): $(CMDS_6809) $(CMDS_68K) $(CMDS_Z80) $(CMDS_GOLF_9) $(CMDS_GOLF_K) $(CMDS_GOLF_Z) $(REPO_DIR)/cmds/os9-6809-level1.zip | $(BUILD_DIR)
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
	rm -f $(REPO_DIR)/cmds/*.9.decb $(REPO_DIR)/cmds/*.k.decb $(REPO_DIR)/cmds/*.z.decb
