package data

import (
	"fmt"
	"strings"
)

type Call struct {
	Name                   string
	Desc                   string
	Number                 byte
	A, B, D, X, Y, U       string
	RA, RB, RD, RX, RY, RU string

	A_off, B_off, D_off, X_off, Y_off, U_off       int
	RA_off, RB_off, RD_off, RX_off, RY_off, RU_off int
}

var Calls = []*Call{
	// User and System State Calls ($00..$20)
	{
		Name:   "F$Link",
		Desc:   "Link to a memory module",
		Number: 0x00,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		RA:     "lang_and_type",
		RB:     "attr_and_rev",
		RX:     "after_module_name",
		RY:     "absolute_entry_addr",
		RU:     "absolute_header_addr",
	},
	{
		Name:   "F$Load",
		Desc:   "Load a module from a file",
		Number: 0x01,
		A:      "lang_and_type_code",
		X:      "module_name_ptr",
		RA:     "lang_and_type",
		RB:     "attr_and_rev",
		RX:     "after_module_name",
		RY:     "absolute_entry_addr",
		RU:     "absolute_header_addr",
	},
	{
		Name:   "F$UnLink",
		Desc:   "Unlink a memory module",
		Number: 0x02,
		U:      "module_hdr_ptr",
	},
	{
		Name:   "F$Fork",
		Desc:   "Create a child process",
		Number: 0x03,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		Y:      "size_of_param_area",
		U:      "start_of_param_area",
		RA:     "child_process_id",
		RX:     "after_module_name",
	},
	{
		Name:   "F$Wait",
		Desc:   "Wait for child process to die",
		Number: 0x04,
		RA:     "child_process_id",
		RB:     "child_exit_status",
	},
	{
		Name:   "F$Chain",
		Desc:   "Chain process to new module",
		Number: 0x05,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		Y:      "size_of_param_area",
		U:      "start_of_param_area",
	},
	{
		Name:   "F$Exit",
		Desc:   "Terminate current process",
		Number: 0x06,
		B:      "status",
	},
	{
		Name:   "F$Mem",
		Desc:   "Set process memory size",
		Number: 0x07,
		D:      "num_bytes_wanted",
		RD:     "actual_num_bytes",
		RY:     "end_of_memory",
	},
	{
		Name:   "F$Send",
		Desc:   "Send signal to process",
		Number: 0x08,
		A:      "process_id",
		B:      "signal_code",
	},
	{
		Name:   "F$Icpt",
		Desc:   "Set intercept signal handler",
		Number: 0x09,
		X:      "signal_handler_addr",
		U:      "data_area_ptr",
	},
	{
		Name:   "F$Sleep",
		Desc:   "Suspend process for duration",
		Number: 0x0A,
		X:      "ticks_to_sleep",
	},
	{
		Name:   "F$SSpnd",
		Desc:   "Suspend calling process",
		Number: 0x0B,
	},
	{
		Name:   "F$ID",
		Desc:   "Get process ID and user ID",
		Number: 0x0C,
		RA:     "process_id",
		RY:     "user_id",
	},
	{
		Name:   "F$SPrior",
		Desc:   "Set process priority",
		Number: 0x0D,
		A:      "process_id",
		B:      "new_priority",
	},
	{
		Name:   "F$SSig",
		Desc:   "Send signal on data ready",
		Number: 0x0E,
		A:      "path_number",
		B:      "signal_code",
	},
	{
		Name:   "F$PErr",
		Desc:   "Print error message",
		Number: 0x0F,
		B:      "error_code",
	},
	{
		Name:   "F$PrsNam",
		Desc:   "Parse path/module name",
		Number: 0x10,
		X:      "name_ptr",
		RA:     "delimiter",
		RB:     "name_length",
		RY:     "after_name_ptr",
	},
	{
		Name:   "F$CmpNam",
		Desc:   "Compare two names",
		Number: 0x11,
		B:      "name_length",
		X:      "name1_ptr",
		Y:      "name2_ptr",
	},
	{
		Name:   "F$Crc",
		Desc:   "Compute CRC checksum",
		Number: 0x12,
		X:      "start_address",
		Y:      "byte_count",
		U:      "crc_accumulator",
		RU:     "new_crc",
	},
	{
		Name:   "F$Time",
		Desc:   "Get system time and date",
		Number: 0x15,
		X:      "time_buffer_ptr",
	},
	{
		Name:   "F$STime",
		Desc:   "Set system time and date",
		Number: 0x16,
		X:      "time_buffer_ptr",
	},
	{
		Name:   "F$TPS",
		Desc:   "Get ticks per second",
		Number: 0x17,
		RD:     "ticks_per_second",
	},

	// System Service Calls ($21..$3E)
	{
		Name:   "F$SRqMem",
		Desc:   "System memory allocate",
		Number: 0x28,
		D:      "byte_count",
		RD:     "actual_num_bytes",
		RU:     "start_address",
	},
	{
		Name:   "F$SRtMem",
		Desc:   "System memory deallocate",
		Number: 0x29,
		D:      "byte_count",
		U:      "start_address",
	},
	{
		Name:   "F$IRQ",
		Desc:   "Install/remove interrupt service",
		Number: 0x2A,
		D:      "device_offset",
		Y:      "service_routine_addr",
		U:      "static_storage_addr",
	},
	{
		Name:   "F$IOQu",
		Desc:   "Queue I/O service request",
		Number: 0x2B,
		A:      "process_id",
	},
	{
		Name:   "F$AProc",
		Desc:   "Insert process into active queue",
		Number: 0x2C,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$NProc",
		Desc:   "Start next active process",
		Number: 0x2D,
	},
	{
		Name:   "F$VModul",
		Desc:   "Validate module header & CRC",
		Number: 0x2E,
		X:      "module_ptr",
		RU:     "module_hdr_ptr",
	},
	{
		Name:   "F$Find64",
		Desc:   "Find 64-byte block / descriptor",
		Number: 0x2F,
		A:      "block_number",
		X:      "base_addr",
		RY:     "block_offset",
	},
	{
		Name:   "F$All64",
		Desc:   "Allocate 64-byte block / descriptor",
		Number: 0x30,
		A:      "start_block",
		B:      "block_count",
		X:      "base_addr",
		RA:     "allocated_block",
		RY:     "block_offset",
	},
	{
		Name:   "F$Ret64",
		Desc:   "Return 64-byte block / descriptor",
		Number: 0x31,
		A:      "block_number",
		X:      "base_addr",
	},
	{
		Name:   "F$SSvc",
		Desc:   "Service request table initialization",
		Number: 0x32,
		Y:      "init_table_addr",
	},
	{
		Name:   "F$IODel",
		Desc:   "Delete I/O module from system",
		Number: 0x33,
		X:      "module_hdr_ptr",
	},
	{
		Name:   "F$SLink",
		Desc:   "System link to module",
		Number: 0x34,
		A:      "module_type",
		X:      "module_name",
		Y:      "name_string_DAT_image_ptr",
	},
	{
		Name:   "F$Boot",
		Desc:   "Bootstrap system devices",
		Number: 0x35,
	},
	{
		Name:   "F$BtMem",
		Desc:   "Allocate bootstrap memory",
		Number: 0x36,
		D:      "byte_count",
		RD:     "actual_num_bytes",
		RY:     "start_address",
	},
	{
		Name:   "F$GProcP",
		Desc:   "Get process pointer",
		Number: 0x37,
		A:      "process_id",
		RY:     "proc_desc_ptr",
	},
	{
		Name:   "F$Move",
		Desc:   "Move data between address spaces",
		Number: 0x38,
		A:      "src_task",
		B:      "dst_task",
		X:      "src_addr",
		Y:      "byte_count",
		U:      "dst_addr",
	},
	{
		Name:   "F$AllRAM",
		Desc:   "Allocate RAM blocks",
		Number: 0x39,
		B:      "block_count",
		RA:     "start_block_number",
	},
	{
		Name:   "F$AllImg",
		Desc:   "Allocate image RAM blocks",
		Number: 0x3A,
		A:      "start_block",
		B:      "block_count",
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$DelImg",
		Desc:   "Deallocate image RAM blocks",
		Number: 0x3B,
		A:      "start_block",
		B:      "block_count",
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$SetImg",
		Desc:   "Set process DAT image",
		Number: 0x3C,
		A:      "start_block",
		B:      "block_count",
		D:      "start_block_number",
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$FreeLB",
		Desc:   "Free low RAM block",
		Number: 0x3D,
		B:      "block_number",
	},
	{
		Name:   "F$FreeHB",
		Desc:   "Free high RAM block",
		Number: 0x3E,
		B:      "block_number",
	},

	// Level 2 / NitrOS-9 specific extensions ($3F..$52)
	{
		Name:   "F$AllTsk",
		Desc:   "Allocate task number",
		Number: 0x3F,
		X:      "proc_desc_addr",
		RA:     "task_number",
	},
	{
		Name:   "F$DelTsk",
		Desc:   "Deallocate task number",
		Number: 0x40,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$SetTsk",
		Desc:   "Set task DAT registers",
		Number: 0x41,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$ResTsk",
		Desc:   "Reserve task number",
		Number: 0x42,
		A:      "task_number",
	},
	{
		Name:   "F$RelTsk",
		Desc:   "Release task number",
		Number: 0x43,
		A:      "task_number",
	},
	{
		Name:   "F$DATRep",
		Desc:   "Replicate DAT image",
		Number: 0x44,
		X:      "src_proc_desc",
		U:      "dst_proc_desc",
	},
	{
		Name:   "F$DATTmp",
		Desc:   "Setup temporary DAT image",
		Number: 0x45,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$LDABX",
		Desc:   "Load A [D+X]",
		Number: 0x46,
		D:      "offset",
		X:      "base_addr",
		RA:     "result",
	},
	{
		Name:   "F$LDAXY",
		Desc:   "Load A [X+Y]",
		Number: 0x47,
		X:      "base_addr",
		Y:      "offset",
		RA:     "result",
	},
	{
		Name:   "F$LDDDXY",
		Desc:   "Load D [D+X,[Y]]",
		Number: 0x48,
		D:      "offset",
		X:      "base_addr",
		Y:      "dat_image_addr",
		RD:     "result",
	},
	{
		Name:   "F$LDABY",
		Desc:   "Load A [D+Y]",
		Number: 0x49,
		D:      "offset",
		Y:      "base_addr",
		RA:     "result",
	},
	{
		Name:   "F$STABX",
		Desc:   "Store A [D+X]",
		Number: 0x4A,
		A:      "value",
		D:      "offset",
		X:      "base_addr",
	},
	{
		Name:   "F$STAXY",
		Desc:   "Store A [X+Y]",
		Number: 0x4B,
		A:      "value",
		X:      "base_addr",
		Y:      "offset",
	},
	{
		Name:   "F$STDDXY",
		Desc:   "Store D [D+X,[Y]]",
		Number: 0x4C,
		D:      "value",
		X:      "base_addr",
		Y:      "dat_image_addr",
	},
	{
		Name:   "F$STABY",
		Desc:   "Store A [D+Y]",
		Number: 0x4D,
		A:      "value",
		D:      "offset",
		Y:      "base_addr",
	},
	{
		Name:   "F$ELink",
		Desc:   "External Link",
		Number: 0x4E,
		X:      "module_name_ptr",
	},
	{
		Name:   "F$FModul",
		Desc:   "Find module in directory",
		Number: 0x4F,
		X:      "proc_desc_addr",
		RA:     "block_number",
		RU:     "module_hdr_ptr",
	},
	{
		Name:   "F$SysDAT",
		Desc:   "Set system DAT image",
		Number: 0x50,
		X:      "dat_image_addr",
	},
	{
		Name:   "F$SetSys",
		Desc:   "Set system task DAT registers",
		Number: 0x51,
	},
	{
		Name:   "F$SRqVec",
		Desc:   "System vector allocation",
		Number: 0x52,
	},

	// I/O Calls ($80..$8F)
	{
		Name:   "I$Attach",
		Desc:   "Attach device to system",
		Number: 0x80,
		A:      "access_mode",
		X:      "device_name",
		RU:     "device_table_ptr",
	},
	{
		Name:   "I$Detach",
		Desc:   "Detach device from system",
		Number: 0x81,
		U:      "device_table_ptr",
	},
	{
		Name:   "I$Dup",
		Desc:   "Duplicate path number",
		Number: 0x82,
		A:      "path",
		RA:     "new_path",
	},
	{
		Name:   "I$Create",
		Desc:   "Create and open new file",
		Number: 0x83,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Open",
		Desc:   "Open existing file or device",
		Number: 0x84,
		A:      "access_mode",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$MakDir",
		Desc:   "Create a new directory",
		Number: 0x85,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$ChgDir",
		Desc:   "Change working directory",
		Number: 0x86,
		A:      "access_mode",
		X:      "pathname",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Delete",
		Desc:   "Delete a file or directory",
		Number: 0x87,
		A:      "access_mode",
		X:      "pathname",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Seek",
		Desc:   "Change current file position pointer",
		Number: 0x88,
		A:      "path",
		X:      "upper_16_bits",
		U:      "lower_16_bits",
	},
	{
		Name:   "I$Read",
		Desc:   "Read raw data bytes",
		Number: 0x89,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$Write",
		Desc:   "Write raw data bytes",
		Number: 0x8A,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$ReadLn",
		Desc:   "Read line of ASCII text",
		Number: 0x8B,
		A:      "path",
		X:      "buffer",
		Y:      "max_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$WritLn",
		Desc:   "Write line of ASCII text",
		Number: 0x8C,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$GetStt",
		Desc:   "Get path/device status",
		Number: 0x8D,
		A:      "path",
		B:      "func_code",
		X:      "param_x",
		Y:      "param_y",
		U:      "param_u",
		RX:     "result_x",
		RY:     "result_y",
		RU:     "result_u",
	},
	{
		Name:   "I$SetStt",
		Desc:   "Set path/device status",
		Number: 0x8E,
		A:      "path",
		B:      "func_code",
		X:      "param_x",
		Y:      "param_y",
		U:      "param_u",
	},
	{
		Name:   "I$Close",
		Desc:   "Close a path",
		Number: 0x8F,
		A:      "path",
	},
}

// OS-9 Status Codes for I$GetStt / I$SetStt
var StatusNames = map[byte]string{
	0x00: "SS.Opt (Read/write PD options)",
	0x01: "SS.Ready (Test for device ready)",
	0x02: "SS.Size (Set file size)",
	0x03: "SS.Reset (Device restore to track 0)",
	0x04: "SS.WTrk (Write track)",
	0x05: "SS.Pos (Get file position)",
	0x06: "SS.EOF (Test for end of file)",
	0x07: "SS.Link (Link status routines)",
	0x08: "SS.ULink (Unlink status routines)",
	0x0A: "SS.Frz (Freeze DD info)",
	0x0B: "SS.SPT (Set tracks/cyl)",
	0x0D: "SS.DCmd (Direct disk command)",
	0x0E: "SS.DevNm (Return device name)",
	0x0F: "SS.FD (Return file descriptor)",
	0x10: "SS.Ticks (Set lockout honor duration)",
	0x11: "SS.Lock (Lock/release record)",
	0x14: "SS.BlkRd (Block read)",
	0x15: "SS.BlkWr (Block write)",
	0x16: "SS.Reten (Retension cycle)",
	0x17: "SS.WFM (Write file mark)",
	0x18: "SS.RFM (Read past file mark)",
	0x19: "SS.ELog (Read error log)",
	0x1A: "SS.SSig (Send signal on data ready)",
	0x1B: "SS.Relea (Release device)",
	0x1E: "SS.RsBit (Reserve bitmap sector)",
	0x20: "SS.FDInf (Get FD sector info)",
	0x26: "SS.ScSiz (Get screen size)",
	0x28: "SS.ComSt (Baud/parity settings)",
	0x29: "SS.Open (Device open notify)",
	0x2A: "SS.Close (Device close notify)",
	0x2B: "SS.HngUp (Hang up phone/modem)",
	0x2C: "SS.FSig (Signal for temp locked file)",
	0xA0: "SS.Fill (Enable command-line history)",
}

var SignalNames = map[byte]string{
	0: "S$Kill (Non-interceptable abort)",
	1: "S$Wake (Wake sleeping process)",
	2: "S$Abort (Keyboard abort / ^C)",
	3: "S$Intrpt (Keyboard interrupt)",
}

var ErrorNames = map[byte]string{
	200: "E$PthFul (Path table full)",
	201: "E$BPNum (Bad path number)",
	202: "E$Poll (Polling table full)",
	203: "E$BMode (Bad mode)",
	204: "E$DevOvf (Device table overflow)",
	205: "E$BMID (Bad module ID)",
	206: "E$DirFul (Module directory full)",
	207: "E$MemFul (Process memory full)",
	208: "E$UnkSvc / E$IllFnc (Unknown service code / Illegal function)",
	209: "E$ModBsy (Module busy)",
	210: "E$BPAddr (Bad page address)",
	211: "E$EOF (End of file)",
	212: "E$VctFul (Vector table full)",
	213: "E$NES / E$Format (Non-existent segment / Bad module format)",
	214: "E$FNA / E$BMode (File not accessible / Bad mode)",
	215: "E$BPNam / E$Proc (Bad path name / Process table full)",
	216: "E$PNNF (Path name not found)",
	217: "E$BPNum / E$SLF (Bad path number / Segment list full)",
	218: "E$PTHFUL / E$CEF (Path table full / Creating existing file)",
	219: "E$IBA (Illegal block address)",
	220: "E$HangUp / E$NoPath (Carrier detect lost / No free path)",
	221: "E$MNF (Module not found)",
	222: "E$NoChld (No children)",
	223: "E$DelSP / E$NotRdy (Deleting SP memory / Child not ready)",
	224: "E$IPrcID / E$ProcUndef (Illegal process ID / Process undefined)",
	225: "E$PrcFul (Process table full)",
	226: "E$NoChld (No children)",
	227: "E$ISWI (Illegal SWI code)",
	228: "E$PrcAbt (Process aborted)",
	229: "E$PrcFul (Process table full)",
	230: "E$IForkP (Illegal fork parameter)",
	231: "E$KwnMod (Known module)",
	232: "E$BMCRC (Bad module CRC)",
	233: "E$USigP (Unprocessed signal pending)",
	234: "E$NEMod (Non-existent module)",
	235: "E$BNam (Bad name)",
	236: "E$BMHP (Bad module header parity)",
	237: "E$NoRAM (No system RAM available)",
	238: "E$DNE (Directory not empty)",
	239: "E$NoTask (No available task number)",
	240: "E$Unit (Illegal media unit)",
	241: "E$Sect (Bad sector number)",
	242: "E$WP (Write protect error)",
	243: "E$CRC (Bad checksum/CRC)",
	244: "E$Read (Read error)",
	245: "E$Write (Write error)",
	246: "E$NotRdy (Device not ready)",
	247: "E$Seek (Seek error)",
	248: "E$Full (Media full)",
	249: "E$BTyp (Bad/incompatible media type)",
	250: "E$DevBsy (Device busy)",
	251: "E$DIDC (Media ID change)",
	252: "E$Lock (Record is busy / locked)",
	253: "E$Share (Non-sharable file busy)",
	254: "E$DeadLk (I/O deadlock error)",
}

var ByNumber [256]*Call

func init() {
	for _, c := range Calls {
		if ByNumber[c.Number] == nil {
			ByNumber[c.Number] = c
		}
	}
	// Hatvan OS / Level 1 conventions:
	ByNumber[0x85] = &Call{
		Name:   "I$Create",
		Desc:   "Create and open new file",
		Number: 0x85,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	}
}

// FindCall returns the Call descriptor for a given syscall number, or nil if unknown.
func FindCall(num byte) *Call {
	return ByNumber[num]
}

// FormatCall returns a pretty-printed string for a TRAP #0 system call invocation in 68000.
// Register conventions in Hatvan OS / M68K:
//
//	D0.B: callNum
//	D1.B: A (path, mode, status, or child PID)
//	A0.L: X (buffer or pathname pointer)
//	D2.L: Y (byte count or size)
//	A1.L: U (parameters pointer)
func FormatCall(c *Call, num byte, d1, d2, a0, a1 uint32, readStr func(addr uint32) string, readBuf func(addr uint32, maxLen int) string) string {
	if c == nil {
		return fmt.Sprintf("TRAP0($%02X)", num)
	}

	var args []string
	if c.A != "" {
		a := byte(d1 & 0xFF)
		if c.A == "path" || c.A == "path_num" || c.A == "file_path" || c.A == "path_number" {
			args = append(args, fmt.Sprintf("path=%d", a))
		} else if strings.Contains(c.A, "mode") {
			args = append(args, fmt.Sprintf("%s=$%02X", c.A, a))
		} else {
			args = append(args, fmt.Sprintf("%s=%d", c.A, a))
		}
	}
	if c.B != "" {
		b := byte(d1 & 0xFF)
		if num == 0x8D || num == 0x8E {
			if stName, ok := StatusNames[b]; ok {
				args = append(args, fmt.Sprintf("func_code=$%02X (%s)", b, stName))
			} else {
				args = append(args, fmt.Sprintf("func_code=$%02X", b))
			}
		} else if num == 0x08 {
			if sigName, ok := SignalNames[b]; ok {
				args = append(args, fmt.Sprintf("signal=$%02X (%s)", b, sigName))
			} else {
				args = append(args, fmt.Sprintf("signal=%d", b))
			}
		} else if c.B == "status" || c.B == "error_code" {
			args = append(args, fmt.Sprintf("%s=%d", c.B, b))
		} else if strings.Contains(c.B, "code") || strings.Contains(c.B, "attr") {
			args = append(args, fmt.Sprintf("%s=$%02X", c.B, b))
		} else {
			args = append(args, fmt.Sprintf("%s=%d", c.B, b))
		}
	}
	if c.D != "" {
		args = append(args, fmt.Sprintf("%s=%d ($%08X)", c.D, d2, d2))
	}
	if c.X != "" {
		if (strings.Contains(c.X, "name") || strings.Contains(c.X, "path")) && readStr != nil {
			str := readStr(a0)
			args = append(args, fmt.Sprintf("%s=$%08X %q", c.X, a0, str))
		} else if c.X == "buffer" {
			args = append(args, fmt.Sprintf("buf=$%08X", a0))
		} else {
			args = append(args, fmt.Sprintf("%s=$%08X", c.X, a0))
		}
	}
	if c.Y != "" {
		if strings.Contains(c.Y, "bytes") || strings.Contains(c.Y, "size") || strings.Contains(c.Y, "count") {
			args = append(args, fmt.Sprintf("%s=%d", c.Y, d2))
		} else {
			args = append(args, fmt.Sprintf("%s=$%08X", c.Y, d2))
		}
	}
	if c.U != "" {
		args = append(args, fmt.Sprintf("%s=$%08X", c.U, a1))
	}

	res := fmt.Sprintf("%s(%s)", c.Name, strings.Join(args, ", "))

	if (num == 0x8A || num == 0x8C) && readBuf != nil && d2 > 0 {
		preview := readBuf(a0, int(d2))
		if preview != "" {
			res += fmt.Sprintf(": %q", preview)
		}
	}

	return res
}

// FormatResult returns a pretty-printed string for a 68000 system call return (after RTE).
// Return conventions in Hatvan OS / M68K:
//
//	SR.C == 0: Success. D1.B = path/child PID, D0.B = status, D2.L = count. Returns "(OK)".
//	SR.C == 1: Error condition. D0.B = error code. Returns "ERROR <num> (<name>)".
func FormatResult(c *Call, num byte, sr uint16, d0, d1, d2, a0, a1 uint32, bufAddr uint32, readBuf func(addr uint32, maxLen int) string) string {
	name := fmt.Sprintf("TRAP0($%02X)", num)
	if c != nil {
		name = c.Name
	}

	isError := (sr & 0x0001) != 0 // Carry set (FlagC)
	if isError {
		errCode := byte(d0 & 0xFF)
		if errName, ok := ErrorNames[errCode]; ok {
			return fmt.Sprintf("%s: ERROR %d (%s)", name, errCode, errName)
		}
		return fmt.Sprintf("%s: ERROR %d", name, errCode)
	}

	if c == nil {
		return fmt.Sprintf("%s: (OK)", name)
	}

	var resParts []string
	if c.RA != "" {
		a := byte(d1 & 0xFF)
		if c.RA == "path" || c.RA == "new_path" || c.RA == "child_process_id" || c.RA == "proc_id" {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RA, a))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=%d ($%02X)", c.RA, a, a))
		}
	}
	if c.RB != "" {
		b := byte(d0 & 0xFF)
		if c.RB == "child_exit_status" {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RB, b))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=%d ($%02X)", c.RB, b, b))
		}
	}
	if c.RD != "" {
		resParts = append(resParts, fmt.Sprintf("%s=%d ($%08X)", c.RD, d2, d2))
	}
	if c.RX != "" {
		resParts = append(resParts, fmt.Sprintf("%s=$%08X", c.RX, a0))
	}
	if c.RY != "" {
		if strings.Contains(c.RY, "bytes") || strings.Contains(c.RY, "size") {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RY, d2))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=$%08X", c.RY, d2))
		}
	}
	if c.RU != "" {
		resParts = append(resParts, fmt.Sprintf("%s=$%08X", c.RU, a1))
	}

	res := ""
	if len(resParts) > 0 {
		res = fmt.Sprintf("%s: %s (OK)", name, strings.Join(resParts, ", "))
	} else {
		res = fmt.Sprintf("%s: (OK)", name)
	}

	if (num == 0x89 || num == 0x8B) && readBuf != nil && d2 > 0 {
		targetAddr := bufAddr
		if targetAddr == 0 {
			targetAddr = a0
		}
		preview := readBuf(targetAddr, int(d2))
		if preview != "" {
			res += fmt.Sprintf(": %q", preview)
		}
	}

	return res
}
