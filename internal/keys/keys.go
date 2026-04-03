package keys

import (
	"bufio"
	"crypto/aes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var keyLineRe = regexp.MustCompile(`(?i)^\s*([a-z0-9_]+)\s*=\s*([A-F0-9]+)\s*$`)

// crc32Checksum maps key names to expected CRC32 of decoded key bytes (Python parity).
var crc32Checksum = map[string]uint32{
	"aes_kek_generation_source":       2545229389,
	"aes_key_generation_source":       459881589,
	"titlekek_source":                 3510501772,
	"key_area_key_application_source": 4130296074,
	"key_area_key_ocean_source":       3975316347,
	"key_area_key_system_source":      4024798875,
	"master_key_00":                   3540309694,
	"master_key_01":                   3477638116,
	"master_key_02":                   2087460235,
	"master_key_03":                   4095912905,
	"master_key_04":                   3833085536,
	"master_key_05":                   2078263136,
	"master_key_06":                   2812171174,
	"master_key_07":                   1146095808,
	"master_key_08":                   1605958034,
	"master_key_09":                   3456782962,
	"master_key_0a":                   2012895168,
	"master_key_0b":                   3813624150,
	"master_key_0c":                   3881579466,
	"master_key_0d":                   723654444,
	"master_key_0e":                   2690905064,
	"master_key_0f":                   4082108335,
	"master_key_10":                   788455323,
	"master_key_11":                   1214507020,
	"master_key_12":                   1051942134,
	"master_key_13":                   2476807835,
}

// Store mirrors Python Keys global state for compatibility.
type Store struct {
	keys                   map[string]string
	titleKeks              []string
	keyAreaKeys            [][][16]byte // [masterIdx][cryptoType 0..2]
	loadedFile             string
	Loaded                 bool
	LoadedKeysRevisions    []string
	IncorrectKeysRevisions []string
	LoadedChecksum         string
}

func NewStore() *Store {
	return &Store{keys: make(map[string]string)}
}

func (s *Store) GetHex(name string) (string, error) {
	v, ok := s.keys[name]
	if !ok {
		return "", fmt.Errorf("%s missing from %s", name, s.loadedFile)
	}
	return v, nil
}

func (s *Store) getKeyBytes(name string) ([]byte, error) {
	h, err := s.GetHex(name)
	if err != nil {
		return nil, err
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	if exp, ok := crc32Checksum[name]; ok {
		if crc32.ChecksumIEEE(b) != exp {
			return nil, fmt.Errorf("%s from %s is invalid (crc32 mismatch)", name, s.loadedFile)
		}
	}
	return b, nil
}

func aesECBDecryptBlock(key, block []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(block) != aes.BlockSize {
		return nil, fmt.Errorf("block size %d", len(block))
	}
	out := make([]byte, aes.BlockSize)
	c.Decrypt(out, block)
	return out, nil
}

func aesECBEncryptBlock(key, block []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, aes.BlockSize)
	c.Encrypt(out, block)
	return out, nil
}

// GenerateKek mirrors Python generateKek.
func GenerateKek(s *Store, src, masterKey, kekSeed, keySeed []byte) ([]byte, error) {
	kek, err := aesECBDecryptBlock(masterKey, kekSeed)
	if err != nil {
		return nil, err
	}
	srcKek, err := aesECBDecryptBlock(kek, src)
	if err != nil {
		return nil, err
	}
	if keySeed == nil {
		return srcKek, nil
	}
	return aesECBDecryptBlock(srcKek, keySeed)
}

func (s *Store) getMasterKey(i int) ([]byte, error) {
	return s.getKeyBytes(fmt.Sprintf("master_key_%02x", i))
}

func existsMasterKey(s *Store, i int) bool {
	_, ok := s.keys[fmt.Sprintf("master_key_%02x", i)]
	return ok
}

// UnwrapAESWrappedTitlekey mirrors Python unwrapAesWrappedTitlekey.
func (s *Store) UnwrapAESWrappedTitlekey(wrappedKey []byte, keyGeneration int) ([]byte, error) {
	aesKekGen, err := s.getKeyBytes("aes_kek_generation_source")
	if err != nil {
		return nil, err
	}
	aesKeyGen, err := s.getKeyBytes("aes_key_generation_source")
	if err != nil {
		return nil, err
	}
	appSrc, err := s.getKeyBytes("key_area_key_application_source")
	if err != nil {
		return nil, err
	}
	masterIdx := keyGeneration
	if masterIdx > 0 {
		masterIdx--
	}
	mk, err := s.getMasterKey(masterIdx)
	if err != nil {
		return nil, err
	}
	kek, err := GenerateKek(s, appSrc, mk, aesKekGen, aesKeyGen)
	if err != nil {
		return nil, err
	}
	if len(wrappedKey) != aes.BlockSize {
		return nil, fmt.Errorf("wrapped key length %d", len(wrappedKey))
	}
	return aesECBDecryptBlock(kek, wrappedKey)
}

// Load parses a prod.keys / keys.txt file.
func (s *Store) Load(path string) (bool, error) {
	s.keys = make(map[string]string)
	s.titleKeks = nil
	s.keyAreaKeys = nil
	s.loadedFile = path
	s.LoadedKeysRevisions = nil
	s.IncorrectKeysRevisions = nil
	s.Loaded = false
	s.LoadedChecksum = ""

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	sum := sha256.New()
	tr := io.TeeReader(f, sum)
	sc := bufio.NewScanner(tr)
	for sc.Scan() {
		line := sc.Text()
		m := keyLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		s.keys[strings.ToLower(m[1])] = m[2]
	}
	if err := sc.Err(); err != nil {
		return false, err
	}
	s.LoadedChecksum = hex.EncodeToString(sum.Sum(nil))

	_, err = s.getKeyBytes("aes_kek_generation_source")
	if err != nil {
		return false, err
	}
	_, err = s.getKeyBytes("aes_key_generation_source")
	if err != nil {
		return false, err
	}
	titlekekSrc, err := s.getKeyBytes("titlekek_source")
	if err != nil {
		return false, err
	}
	kApp, err := s.getKeyBytes("key_area_key_application_source")
	if err != nil {
		return false, err
	}
	kOcean, err := s.getKeyBytes("key_area_key_ocean_source")
	if err != nil {
		return false, err
	}
	kSys, err := s.getKeyBytes("key_area_key_system_source")
	if err != nil {
		return false, err
	}

	aesKekGen, _ := s.getKeyBytes("aes_kek_generation_source")
	aesKeyGen, _ := s.getKeyBytes("aes_key_generation_source")

	s.keyAreaKeys = make([][][16]byte, 32)
	for i := range s.keyAreaKeys {
		s.keyAreaKeys[i] = make([][16]byte, 3)
	}

	for i := 0; i < 32; i++ {
		if !existsMasterKey(s, i) {
			continue
		}
		masterKey, err := s.getMasterKey(i)
		if err != nil {
			s.IncorrectKeysRevisions = append(s.IncorrectKeysRevisions, fmt.Sprintf("master_key_%02x", i))
			continue
		}
		tk, err := aesECBDecryptBlock(masterKey, titlekekSrc)
		if err != nil {
			s.IncorrectKeysRevisions = append(s.IncorrectKeysRevisions, fmt.Sprintf("master_key_%02x", i))
			continue
		}
		s.titleKeks = append(s.titleKeks, hex.EncodeToString(tk))

		ka0, err := GenerateKek(s, kApp, masterKey, aesKekGen, aesKeyGen)
		if err != nil {
			s.IncorrectKeysRevisions = append(s.IncorrectKeysRevisions, fmt.Sprintf("master_key_%02x", i))
			continue
		}
		ka1, err := GenerateKek(s, kOcean, masterKey, aesKekGen, aesKeyGen)
		if err != nil {
			s.IncorrectKeysRevisions = append(s.IncorrectKeysRevisions, fmt.Sprintf("master_key_%02x", i))
			continue
		}
		ka2, err := GenerateKek(s, kSys, masterKey, aesKekGen, aesKeyGen)
		if err != nil {
			s.IncorrectKeysRevisions = append(s.IncorrectKeysRevisions, fmt.Sprintf("master_key_%02x", i))
			continue
		}
		copy(s.keyAreaKeys[i][0][:], ka0)
		copy(s.keyAreaKeys[i][1][:], ka1)
		copy(s.keyAreaKeys[i][2][:], ka2)
		s.LoadedKeysRevisions = append(s.LoadedKeysRevisions, fmt.Sprintf("master_key_%02x", i))
	}

	if len(s.IncorrectKeysRevisions) > 0 {
		s.Loaded = false
		return false, nil
	}
	s.Loaded = true
	return true, nil
}

// DefaultKeySearchPaths returns candidate key file paths (Python load_default order).
func DefaultKeySearchPaths(execDir string, home string) []string {
	return []string{
		filepath.Join(execDir, "prod.keys"),
		filepath.Join(execDir, "keys.txt"),
		filepath.Join(home, ".switch", "prod.keys"),
		filepath.Join(home, ".switch", "keys.txt"),
	}
}

// LoadDefault tries default locations.
func (s *Store) LoadDefault(execDir, home string) error {
	for _, p := range DefaultKeySearchPaths(execDir, home) {
		ok, err := s.Load(p)
		if err != nil {
			continue
		}
		if ok {
			return nil
		}
	}
	return fmt.Errorf("failed to load keys from prod.keys or keys.txt (searched %v)", DefaultKeySearchPaths(execDir, home))
}
