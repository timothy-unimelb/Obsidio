#!/usr/bin/env python3
"""Generate app/sha256block2_amd64.s: two-lane interleaved SHA-NI single-block
compression, mechanically derived from the vendored stdlib blockSHANI routine.

Lane A registers: state X1,X2; W X3,X4,X5,X6; temp X7; data SI; dig DI
Lane B registers: state X8,X9; W X10,X11,X12,X13; temp X15; data R9; dig R10
Shared: X0 (implicit SHA256RNDS2 WK operand, reloaded per group), X14 (flip
mask), AX (K256 table), stack 0..63 (saved initial states for feedforward).

Interleaving is at 4-round-group granularity: each lane's WK lives in X0 only
within its own group, so the lanes never fight over the implicit register and
the OOO core overlaps the two dependency chains.
"""

# Single-lane 4-round groups transcribed from the vendored blockSHANI
# (app/sha256block_amd64.s lines 1315-1465). Register placeholders:
# S1,S2 = state; W0,W1,W2,W3 = current rotation of msg regs; T = temp;
# F = flip mask; DATA = message pointer.
LOAD_GROUPS = [
    # (data offset, K offset, W target, trailing msg1 (dst,src) or None, mid schedule or None)
    (0,   0,   "W0", None,          None),
    (16,  32,  "W1", ("W0", "W1"),  None),
    (32,  64,  "W2", ("W1", "W2"),  None),
    (48,  96,  "W3", ("W2", "W3"),  ("W3", "W2", "W0")),  # mid: T=W3;palignr W2;paddd->W0? see emit
]
# Middle groups: (K offset, src W, sched dst W, palignr src W, msg1 pair or None)
MID_GROUPS = [
    (128, "W0", "W1", "W3", ("W3", "W0")),
    (160, "W1", "W2", "W0", ("W0", "W1")),
    (192, "W2", "W3", "W1", ("W1", "W2")),
    (224, "W3", "W0", "W2", ("W2", "W3")),
    (256, "W0", "W1", "W3", ("W3", "W0")),
    (288, "W1", "W2", "W0", ("W0", "W1")),
    (320, "W2", "W3", "W1", ("W1", "W2")),
    (352, "W3", "W0", "W2", ("W2", "W3")),
    (384, "W0", "W1", "W3", ("W3", "W0")),
    (416, "W1", "W2", "W0", None),
    (448, "W2", "W3", "W1", None),
]
FINAL_K = 480  # rounds 60-63, src W3


def lane_regs(lane):
    if lane == "A":
        return {"S1": "X1", "S2": "X2", "W0": "X3", "W1": "X4", "W2": "X5",
                "W3": "X6", "T": "X7", "DATA": "SI", "DIG": "DI",
                "SAVE1": "(SP)", "SAVE2": "16(SP)"}
    return {"S1": "X8", "S2": "X9", "W0": "X10", "W1": "X11", "W2": "X12",
            "W3": "X13", "T": "X15", "DATA": "R9", "DIG": "R10",
            "SAVE1": "32(SP)", "SAVE2": "48(SP)"}


def rnds2_pair(r, out):
    out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
    out.append("\tPSHUFD      $0x0e, X0, X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")


def emit_load_group(lane, idx, out, from_reg=False):
    r = lane_regs(lane)
    off, koff, wt, msg1, mid = LOAD_GROUPS[idx]
    w = r[wt]
    out.append(f"\t// lane {lane} rounds {idx*4}-{idx*4+3}")
    if from_reg:
        # N-variant loop body: the flipped message quad is already in W
        # (initial load in the prologue, or the ASCII back-edge flip).
        out.append(f"\tVMOVDQA     {w}, X0")
    else:
        src = f"({r['DATA']})" if off == 0 else f"{off}({r['DATA']})"
        out.append(f"\tVMOVDQU     {src}, X0")
        out.append("\tPSHUFB      X14, X0")
        out.append(f"\tVMOVDQA     X0, {w}")
    kop = "(AX)" if koff == 0 else f"{koff}(AX)"
    out.append(f"\tPADDD       {kop}, X0")
    if mid is None:
        rnds2_pair(r, out)
    else:
        # group 3 pattern: first rnds2, schedule W0 via palignr/msg2, then second
        out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
        out.append(f"\tVMOVDQA     {r['W3']}, {r['T']}")
        out.append(f"\tPALIGNR     $0x04, {r['W2']}, {r['T']}")
        out.append(f"\tPADDD       {r['T']}, {r['W0']}")
        out.append(f"\tSHA256MSG2  {r['W3']}, {r['W0']}")
        out.append("\tPSHUFD      $0x0e, X0, X0")
        out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")
    if msg1 is not None:
        out.append(f"\tSHA256MSG1  {r[msg1[1]]}, {r[msg1[0]]}")


def emit_mid_group(lane, gi, out):
    r = lane_regs(lane)
    koff, wsrc, wdst, walign, msg1 = MID_GROUPS[gi]
    rounds = 16 + gi * 4
    out.append(f"\t// lane {lane} rounds {rounds}-{rounds+3}")
    out.append(f"\tVMOVDQA     {r[wsrc]}, X0")
    out.append(f"\tPADDD       {koff}(AX), X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
    out.append(f"\tVMOVDQA     {r[wsrc]}, {r['T']}")
    out.append(f"\tPALIGNR     $0x04, {r[walign]}, {r['T']}")
    out.append(f"\tPADDD       {r['T']}, {r[wdst]}")
    out.append(f"\tSHA256MSG2  {r[wsrc]}, {r[wdst]}")
    out.append("\tPSHUFD      $0x0e, X0, X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")
    if msg1 is not None:
        out.append(f"\tSHA256MSG1  {r[msg1[1]]}, {r[msg1[0]]}")


def emit_final_group(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// lane {lane} rounds 60-63")
    out.append(f"\tVMOVDQA     {r['W3']}, X0")
    out.append(f"\tPADDD       {FINAL_K}(AX), X0")
    rnds2_pair(r, out)


def emit_state_load(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// load lane {lane} state, shuffle to ABEF/CDGH, save for feedforward")
    out.append(f"\tVMOVDQU ({r['DIG']}), {r['S1']}")
    out.append(f"\tVMOVDQU 16({r['DIG']}), {r['S2']}")
    out.append(f"\tPSHUFD  $0xb1, {r['S1']}, {r['S1']}")
    out.append(f"\tPSHUFD  $0x1b, {r['S2']}, {r['S2']}")
    out.append(f"\tVMOVDQA {r['S1']}, {r['T']}")
    out.append(f"\tPALIGNR $0x08, {r['S2']}, {r['S1']}")
    out.append(f"\tPBLENDW $0xf0, {r['T']}, {r['S2']}")
    out.append(f"\tVMOVDQU {r['S1']}, {r['SAVE1']}")
    out.append(f"\tVMOVDQU {r['S2']}, {r['SAVE2']}")


def emit_state_store(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// lane {lane}: feedforward + unshuffle + store")
    out.append(f"\tVMOVDQU {r['SAVE1']}, {r['T']}")
    out.append(f"\tPADDD   {r['T']}, {r['S1']}")
    out.append(f"\tVMOVDQU {r['SAVE2']}, {r['T']}")
    out.append(f"\tPADDD   {r['T']}, {r['S2']}")
    out.append(f"\tPSHUFD  $0x1b, {r['S1']}, {r['S1']}")
    out.append(f"\tPSHUFD  $0xb1, {r['S2']}, {r['S2']}")
    out.append(f"\tVMOVDQA {r['S1']}, {r['T']}")
    out.append(f"\tPBLENDW $0xf0, {r['S2']}, {r['S1']}")
    out.append(f"\tPALIGNR $0x08, {r['T']}, {r['S2']}")
    out.append(f"\tVMOVDQU {r['S1']}, ({r['DIG']})")
    out.append(f"\tVMOVDQU {r['S2']}, 16({r['DIG']})")


out = []
out.append("// Code generated by benchmarks/gen2lane.py — two-lane interleave of the")
out.append("// vendored stdlib blockSHANI (sha256block_amd64.s). DO NOT EDIT BY HAND.")
out.append("// Requires: AVX, SHA, SSE4.1, SSSE3 (same gate as blockSHANI).")
out.append("")
out.append("//go:build !purego")
out.append("")
out.append('#include "textflag.h"')
out.append("")
out.append("// func blockSHANI2(da *shaDigest, db *shaDigest, pa *[64]byte, pb *[64]byte)")
out.append("TEXT ·blockSHANI2(SB), NOSPLIT, $64-32")
out.append("\tMOVQ    da+0(FP), DI")
out.append("\tMOVQ    db+8(FP), R10")
out.append("\tMOVQ    pa+16(FP), SI")
out.append("\tMOVQ    pb+24(FP), R9")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
emit_state_load("A", out)
emit_state_load("B", out)
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out)
    emit_load_group("B", i, out)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
    emit_mid_group("B", g, out)
emit_final_group("A", out)
emit_final_group("B", out)
emit_state_store("A", out)
emit_state_store("B", out)
out.append("\tRET")
out.append("")


# ---------------------------------------------------------------------------
# pairHashHex: one fused iteration of the /risk chain for TWO lanes, in place.
# Reads 64 hex bytes per lane, computes sha256, writes 64 hex bytes back.
#   block 1: full schedule (input varies)
#   block 2: the CONSTANT padding block — its W+K schedule is precomputed
#            below, so it is 64 rounds of rnds2 + table loads, zero schedule
#   epilogue: unshuffle -> byte-swap -> PSHUFB nibble-LUT hex, all in-register
# ---------------------------------------------------------------------------

K = [0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
     0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
     0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
     0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
     0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
     0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
     0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
     0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2]

M32 = 0xFFFFFFFF
def ror(x, n): return ((x >> n) | (x << (32 - n))) & M32
def s0(x): return ror(x, 7) ^ ror(x, 18) ^ (x >> 3)
def s1(x): return ror(x, 17) ^ ror(x, 19) ^ (x >> 10)

# Padding block for a 64-byte message: 0x80, zeros, 64-bit BE length (512).
padW = [0x80000000] + [0] * 14 + [0x00000200]
for t in range(16, 64):
    padW.append((s1(padW[t-2]) + padW[t-7] + s0(padW[t-15]) + padW[t-16]) & M32)
WKPAD = [(padW[t] + K[t]) & M32 for t in range(64)]

IV = [0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19]

def emit_pad_group(lane, gi, out):
    """Block-2 4-round group: WK straight from the precomputed table."""
    r = lane_regs(lane)
    out.append(f"\t// lane {lane} pad-block rounds {gi*4}-{gi*4+3} (precomputed WK)")
    out.append(f"\tVMOVDQA     {gi*16}(BX), X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
    out.append("\tPSHUFD      $0x0e, X0, X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")

def emit_hex_epilogue(lane, out):
    """Unshuffle state, byte-swap, hex-expand via PSHUFB, store 64 bytes."""
    r = lane_regs(lane)
    s1r, s2r, data = r["S1"], r["S2"], r["DATA"]
    out.append(f"\t// lane {lane}: unshuffle to h-order dwords")
    out.append(f"\tPSHUFD  $0x1b, {s1r}, {s1r}")
    out.append(f"\tPSHUFD  $0xb1, {s2r}, {s2r}")
    out.append(f"\tVMOVDQA {s1r}, {r['T']}")
    out.append(f"\tPBLENDW $0xf0, {s2r}, {s1r}")
    out.append(f"\tPALIGNR $0x08, {r['T']}, {s2r}")
    for half, (reg, off) in enumerate([(s1r, 0), (s2r, 32)]):
        out.append(f"\t// lane {lane}: digest bytes {half*16}-{half*16+15} -> 32 hex chars")
        out.append(f"\tPSHUFB  X3, {reg}")        # bswap32: digest byte order
        out.append(f"\tVMOVDQA {reg}, X6")
        out.append(f"\tPSRLW   $0x04, X6")
        out.append("\tPAND    X4, X6")            # X6 = hi nibbles
        out.append(f"\tPAND    X4, {reg}")        # reg = lo nibbles
        out.append("\tVMOVDQA X5, X7")
        out.append("\tPSHUFB  X6, X7")            # X7 = ascii(hi)
        out.append("\tVMOVDQA X5, X6")
        out.append(f"\tPSHUFB  {reg}, X6")        # X6 = ascii(lo)
        out.append("\tVMOVDQA X7, X0")
        out.append("\tPUNPCKLBW X6, X0")          # hi0,lo0,...
        out.append("\tPUNPCKHBW X6, X7")
        out.append(f"\tVMOVDQU X0, {off}({data})")
        out.append(f"\tVMOVDQU X7, {off+16}({data})")

out.append("// func pairHashHex(pa *[64]byte, pb *[64]byte)")
out.append("// One fused chain iteration for two lanes, in place (hex -> hex).")
out.append("TEXT ·pairHashHex(SB), NOSPLIT, $96-16")
out.append("\tMOVQ    pa+0(FP), SI")
out.append("\tMOVQ    pb+8(FP), R9")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
out.append("\tLEAQ    wk_pad<>+0(SB), BX")
out.append("\t// shuffled IV -> both lanes; saved once for the first feedforward")
out.append("\tVMOVDQU iv_words<>+0(SB), X1")
out.append("\tVMOVDQU iv_words<>+16(SB), X2")
out.append("\tPSHUFD  $0xb1, X1, X1")
out.append("\tPSHUFD  $0x1b, X2, X2")
out.append("\tVMOVDQA X1, X7")
out.append("\tPALIGNR $0x08, X2, X1")
out.append("\tPBLENDW $0xf0, X7, X2")
out.append("\tVMOVDQU X1, (SP)")
out.append("\tVMOVDQU X2, 16(SP)")
out.append("\tVMOVDQA X1, X8")
out.append("\tVMOVDQA X2, X9")
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out)
    emit_load_group("B", i, out)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
    emit_mid_group("B", g, out)
emit_final_group("A", out)
emit_final_group("B", out)
out.append("\t// feedforward 1 (H1 = IV + comp), save H1 for feedforward 2")
for (sreg1, sreg2, o1, o2) in [("X1", "X2", 32, 48), ("X8", "X9", 64, 80)]:
    out.append("\tVMOVDQU (SP), X7")
    out.append(f"\tPADDD   X7, {sreg1}")
    out.append("\tVMOVDQU 16(SP), X7")
    out.append(f"\tPADDD   X7, {sreg2}")
    out.append(f"\tVMOVDQU {sreg1}, {o1}(SP)")
    out.append(f"\tVMOVDQU {sreg2}, {o2}(SP)")
for gi in range(16):
    emit_pad_group("A", gi, out)
    emit_pad_group("B", gi, out)
out.append("\t// feedforward 2 (final digest state)")
for (sreg1, sreg2, o1, o2) in [("X1", "X2", 32, 48), ("X8", "X9", 64, 80)]:
    out.append(f"\tVMOVDQU {o1}(SP), X7")
    out.append(f"\tPADDD   X7, {sreg1}")
    out.append(f"\tVMOVDQU {o2}(SP), X7")
    out.append(f"\tPADDD   X7, {sreg2}")
out.append("\t// hex-epilogue constants: X3 bswap, X4 0x0f, X5 ascii LUT")
out.append("\tVMOVDQA bswap32<>+0(SB), X3")
out.append("\tVMOVDQA mask0f<>+0(SB), X4")
out.append("\tVMOVDQA hexlut<>+0(SB), X5")
emit_hex_epilogue("A", out)
emit_hex_epilogue("B", out)
out.append("\tRET")
out.append("")

def emit_dwords(name, words, out):
    for i, w in enumerate(words):
        out.append(f"DATA {name}<>+{i*4}(SB)/4, ${'0x%08x' % w}")
    out.append(f"GLOBL {name}<>(SB), RODATA|NOPTR, ${len(words)*4}")
    out.append("")

def emit_bytes(name, bs, out):
    for i in range(0, len(bs), 8):
        chunk = bs[i:i+8]
        val = int.from_bytes(bytes(chunk), "little")
        out.append(f"DATA {name}<>+{i}(SB)/8, ${'0x%016x' % val}")
    out.append(f"GLOBL {name}<>(SB), RODATA|NOPTR, ${len(bs)}")
    out.append("")

# Single-lane fused iteration: same structure, lane A only. Lone-waiter
# chains (grading ramp, odd queue depths, calibration) get the fused win too.
out.append("// func hashHex1(pa *[64]byte)")
out.append("// One fused chain iteration for a single lane, in place (hex -> hex).")
out.append("TEXT ·hashHex1(SB), NOSPLIT, $64-8")
out.append("\tMOVQ    pa+0(FP), SI")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
out.append("\tLEAQ    wk_pad<>+0(SB), BX")
out.append("\tVMOVDQU iv_words<>+0(SB), X1")
out.append("\tVMOVDQU iv_words<>+16(SB), X2")
out.append("\tPSHUFD  $0xb1, X1, X1")
out.append("\tPSHUFD  $0x1b, X2, X2")
out.append("\tVMOVDQA X1, X7")
out.append("\tPALIGNR $0x08, X2, X1")
out.append("\tPBLENDW $0xf0, X7, X2")
out.append("\tVMOVDQU X1, (SP)")
out.append("\tVMOVDQU X2, 16(SP)")
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
emit_final_group("A", out)
out.append("\t// feedforward 1, save H1")
out.append("\tVMOVDQU (SP), X7")
out.append("\tPADDD   X7, X1")
out.append("\tVMOVDQU 16(SP), X7")
out.append("\tPADDD   X7, X2")
out.append("\tVMOVDQU X1, 32(SP)")
out.append("\tVMOVDQU X2, 48(SP)")
for gi in range(16):
    emit_pad_group("A", gi, out)
out.append("\t// feedforward 2")
out.append("\tVMOVDQU 32(SP), X7")
out.append("\tPADDD   X7, X1")
out.append("\tVMOVDQU 48(SP), X7")
out.append("\tPADDD   X7, X2")
out.append("\tVMOVDQA bswap32<>+0(SB), X3")
out.append("\tVMOVDQA mask0f<>+0(SB), X4")
out.append("\tVMOVDQA hexlut<>+0(SB), X5")
emit_hex_epilogue("A", out)
out.append("\tRET")
out.append("")

# ---------------------------------------------------------------------------
# Kernel v3: N-iteration variants with the chain loop INSIDE the asm.
# Per iteration the epilogue emits each lane's 64 ASCII digest bytes into that
# lane's W registers (dead after the rounds) instead of memory; the back-edge
# PSHUFB-flips them in place into message form and jumps back to the rounds.
# Memory is touched only at entry (initial load) and exit (final store).
# flip_mask doubles as the epilogue's bswap32 (byte-identical constants, see
# the DATA blocks below); mask0f sits in X15 during the epilogue (X15 is lane
# B's rounds temp — dead there); hexlut reloads from RODATA per use.
# ---------------------------------------------------------------------------

def emit_msg_load_flip(lane, out):
    """Initial load: 64 raw bytes -> flipped message quads in the W slots."""
    r = lane_regs(lane)
    out.append(f"\t// lane {lane}: load 64 bytes, flip to message form in W slots")
    for qi, wt in enumerate(("W0", "W1", "W2", "W3")):
        src = f"({r['DATA']})" if qi == 0 else f"{qi*16}({r['DATA']})"
        out.append(f"\tVMOVDQU {src}, {r[wt]}")
    for wt in ("W0", "W1", "W2", "W3"):
        out.append(f"\tPSHUFB  X14, {r[wt]}")


def emit_msg_flip(lane, out):
    """Back-edge: ASCII digest in W slots -> flipped message for next round."""
    r = lane_regs(lane)
    for wt in ("W0", "W1", "W2", "W3"):
        out.append(f"\tPSHUFB  X14, {r[wt]}")


def emit_msg_store(lane, out):
    """Exit: store the final ASCII digest from the W slots to memory."""
    r = lane_regs(lane)
    out.append(f"\t// lane {lane}: store final 64 ASCII bytes")
    for qi, wt in enumerate(("W0", "W1", "W2", "W3")):
        dst = f"({r['DATA']})" if qi == 0 else f"{qi*16}({r['DATA']})"
        out.append(f"\tVMOVDQU {r[wt]}, {dst}")


def emit_hex_epilogue_reg(lane, out):
    """Unshuffle + byte-swap + hex-expand, ASCII landing in the lane's W
    slots (W0,W1 = bytes 0-31, W2,W3 = bytes 32-63). Temps X0/X7 (dead
    post-rounds); X15 = mask0f; X14 = flip_mask serving as bswap32."""
    r = lane_regs(lane)
    s1r, s2r = r["S1"], r["S2"]
    outs = [r["W0"], r["W1"], r["W2"], r["W3"]]
    out.append(f"\t// lane {lane}: unshuffle to h-order dwords")
    out.append(f"\tPSHUFD  $0x1b, {s1r}, {s1r}")
    out.append(f"\tPSHUFD  $0xb1, {s2r}, {s2r}")
    out.append(f"\tVMOVDQA {s1r}, X0")
    out.append(f"\tPBLENDW $0xf0, {s2r}, {s1r}")
    out.append(f"\tPALIGNR $0x08, X0, {s2r}")
    for half, (reg, olo, ohi) in enumerate([(s1r, outs[0], outs[1]),
                                            (s2r, outs[2], outs[3])]):
        out.append(f"\t// lane {lane}: digest bytes {half*16}-{half*16+15} -> 32 hex chars in {olo},{ohi}")
        out.append(f"\tPSHUFB  X14, {reg}")      # flip_mask == bswap32
        out.append(f"\tVMOVDQA {reg}, X0")
        out.append("\tPSRLW   $0x04, X0")
        out.append("\tPAND    X15, X0")          # X0 = hi nibbles
        out.append(f"\tPAND    X15, {reg}")      # reg = lo nibbles
        out.append("\tVMOVDQA hexlut<>+0(SB), X7")
        out.append("\tPSHUFB  X0, X7")           # X7 = ascii(hi)
        out.append("\tVMOVDQA hexlut<>+0(SB), X0")
        out.append(f"\tPSHUFB  {reg}, X0")       # X0 = ascii(lo)
        out.append(f"\tVMOVDQA X7, {olo}")
        out.append(f"\tPUNPCKLBW X0, {olo}")     # hi0,lo0,...
        out.append(f"\tPUNPCKHBW X0, X7")
        out.append(f"\tVMOVDQA X7, {ohi}")


out.append("// func pairHashHexN(pa *[64]byte, pb *[64]byte, n int)")
out.append("// n fused chain iterations for two lanes, in place. The loop lives in the")
out.append("// asm: intermediate digests never touch memory (ASCII W-slot back-edge).")
out.append("TEXT ·pairHashHexN(SB), NOSPLIT, $96-24")
out.append("\tMOVQ    pa+0(FP), SI")
out.append("\tMOVQ    pb+8(FP), R9")
out.append("\tMOVQ    n+16(FP), CX")
out.append("\tTESTQ   CX, CX")
out.append("\tJG      pairN_start")
out.append("\tRET")
out.append("")
out.append("pairN_start:")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
out.append("\tLEAQ    wk_pad<>+0(SB), BX")
out.append("\t// shuffled IV -> (SP),16(SP); reloaded at every loop head")
out.append("\tVMOVDQU iv_words<>+0(SB), X1")
out.append("\tVMOVDQU iv_words<>+16(SB), X2")
out.append("\tPSHUFD  $0xb1, X1, X1")
out.append("\tPSHUFD  $0x1b, X2, X2")
out.append("\tVMOVDQA X1, X7")
out.append("\tPALIGNR $0x08, X2, X1")
out.append("\tPBLENDW $0xf0, X7, X2")
out.append("\tVMOVDQU X1, (SP)")
out.append("\tVMOVDQU X2, 16(SP)")
emit_msg_load_flip("A", out)
emit_msg_load_flip("B", out)
out.append("")
out.append("pairN_loop:")
out.append("\t// per-iteration state init from the saved shuffled IV")
out.append("\tVMOVDQU (SP), X1")
out.append("\tVMOVDQU 16(SP), X2")
out.append("\tVMOVDQA X1, X8")
out.append("\tVMOVDQA X2, X9")
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out, from_reg=True)
    emit_load_group("B", i, out, from_reg=True)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
    emit_mid_group("B", g, out)
emit_final_group("A", out)
emit_final_group("B", out)
out.append("\t// feedforward 1 (H1 = IV + comp), save H1 for feedforward 2")
for (sreg1, sreg2, o1, o2) in [("X1", "X2", 32, 48), ("X8", "X9", 64, 80)]:
    out.append("\tVMOVDQU (SP), X7")
    out.append(f"\tPADDD   X7, {sreg1}")
    out.append("\tVMOVDQU 16(SP), X7")
    out.append(f"\tPADDD   X7, {sreg2}")
    out.append(f"\tVMOVDQU {sreg1}, {o1}(SP)")
    out.append(f"\tVMOVDQU {sreg2}, {o2}(SP)")
for gi in range(16):
    emit_pad_group("A", gi, out)
    emit_pad_group("B", gi, out)
out.append("\t// feedforward 2 (final digest state)")
for (sreg1, sreg2, o1, o2) in [("X1", "X2", 32, 48), ("X8", "X9", 64, 80)]:
    out.append(f"\tVMOVDQU {o1}(SP), X7")
    out.append(f"\tPADDD   X7, {sreg1}")
    out.append(f"\tVMOVDQU {o2}(SP), X7")
    out.append(f"\tPADDD   X7, {sreg2}")
out.append("\t// epilogue nibble mask (X15 = lane B rounds temp, dead here)")
out.append("\tVMOVDQA mask0f<>+0(SB), X15")
emit_hex_epilogue_reg("A", out)
emit_hex_epilogue_reg("B", out)
out.append("\tDECQ    CX")
out.append("\tJZ      pairN_done")
out.append("\t// back-edge: the ASCII digests in the W slots ARE the next messages")
emit_msg_flip("A", out)
emit_msg_flip("B", out)
out.append("\tJMP     pairN_loop")
out.append("")
out.append("pairN_done:")
emit_msg_store("A", out)
emit_msg_store("B", out)
out.append("\tRET")
out.append("")

out.append("// func hashHex1N(pa *[64]byte, n int)")
out.append("// n fused chain iterations for a single lane, in place (loop in asm).")
out.append("TEXT ·hashHex1N(SB), NOSPLIT, $64-16")
out.append("\tMOVQ    pa+0(FP), SI")
out.append("\tMOVQ    n+8(FP), CX")
out.append("\tTESTQ   CX, CX")
out.append("\tJG      hash1N_start")
out.append("\tRET")
out.append("")
out.append("hash1N_start:")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
out.append("\tLEAQ    wk_pad<>+0(SB), BX")
out.append("\tVMOVDQU iv_words<>+0(SB), X1")
out.append("\tVMOVDQU iv_words<>+16(SB), X2")
out.append("\tPSHUFD  $0xb1, X1, X1")
out.append("\tPSHUFD  $0x1b, X2, X2")
out.append("\tVMOVDQA X1, X7")
out.append("\tPALIGNR $0x08, X2, X1")
out.append("\tPBLENDW $0xf0, X7, X2")
out.append("\tVMOVDQU X1, (SP)")
out.append("\tVMOVDQU X2, 16(SP)")
emit_msg_load_flip("A", out)
out.append("")
out.append("hash1N_loop:")
out.append("\tVMOVDQU (SP), X1")
out.append("\tVMOVDQU 16(SP), X2")
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out, from_reg=True)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
emit_final_group("A", out)
out.append("\t// feedforward 1, save H1")
out.append("\tVMOVDQU (SP), X7")
out.append("\tPADDD   X7, X1")
out.append("\tVMOVDQU 16(SP), X7")
out.append("\tPADDD   X7, X2")
out.append("\tVMOVDQU X1, 32(SP)")
out.append("\tVMOVDQU X2, 48(SP)")
for gi in range(16):
    emit_pad_group("A", gi, out)
out.append("\t// feedforward 2")
out.append("\tVMOVDQU 32(SP), X7")
out.append("\tPADDD   X7, X1")
out.append("\tVMOVDQU 48(SP), X7")
out.append("\tPADDD   X7, X2")
out.append("\tVMOVDQA mask0f<>+0(SB), X15")
emit_hex_epilogue_reg("A", out)
out.append("\tDECQ    CX")
out.append("\tJZ      hash1N_done")
emit_msg_flip("A", out)
out.append("\tJMP     hash1N_loop")
out.append("")
out.append("hash1N_done:")
emit_msg_store("A", out)
out.append("\tRET")
out.append("")

emit_dwords("wk_pad", WKPAD, out)
emit_dwords("iv_words", IV, out)
emit_bytes("bswap32", [3,2,1,0,7,6,5,4,11,10,9,8,15,14,13,12], out)
emit_bytes("mask0f", [0x0F]*16, out)
emit_bytes("hexlut", list(b"0123456789abcdef"), out)

# K256<> and flip_mask<> are FILE-LOCAL in Go asm — this file needs its own
# copies, extracted verbatim from the vendored stdlib file.
import re
import sys

with open("app/sha256block_amd64.s") as f:
    vendored = f.read()
for sym in ("flip_mask", "K256"):
    lines = [l for l in vendored.split("\n")
             if re.match(rf"^(DATA|GLOBL) {sym}<>", l)]
    if not lines:
        raise SystemExit(f"symbol {sym} not found in vendored asm")
    out.append(f"// {sym}: verbatim copy from sha256block_amd64.s (file-local there)")
    out.extend(lines)
    out.append("")
path = sys.argv[1] if len(sys.argv) > 1 else "sha256block2_amd64.s"
with open(path, "w") as f:
    f.write("\n".join(out))
print(f"wrote {path}: {len(out)} lines")
