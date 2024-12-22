package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestVDF(t *testing.T) {
	// define a seed
	seed := []byte("seed")
	// create a new vdf service
	vdfService := &VDFService{running: &atomic.Bool{}, Iterations: 1000}
	// generates a VDF proof using the current params state of the VDF Service object
	vdfService.Run(seed)
	// finish the vdf
	results := vdfService.Finish()
	// verify the vdf
	require.True(t, vdfService.VerifyVDF(seed, results))
}

func TestVDFPrematureExit(t *testing.T) {
	testTimeout := 2 * time.Second
	// define a seed
	seed, iterations := []byte("seed"), 1000
	// create a new vdf service
	vdfService := &VDFService{TargetTime: time.Second, stopChan: make(chan struct{}), running: &atomic.Bool{}, Iterations: iterations}
	// generates a VDF proof using the current params state of the VDF Service object
	go vdfService.Run(seed)
out:
	for {
		select {
		case <-time.After(testTimeout):
			t.Fatal("test timeout")
		default:
			if vdfService.running.Load() == true {
				break out
			}
		}
	}
	// exit the vdf immediately
	require.Nil(t, vdfService.Finish())
loop:
	for {
		select {
		case <-time.After(testTimeout):
			t.Fatal("test timeout")
		default:
			if vdfService.Iterations != iterations {
				break loop
			}
		}
	}
	// ensure empty results
	require.EqualExportedValues(t, VDF{}, vdfService.Results)
	// ensure iterations were adjusted smaller
	require.Less(t, vdfService.Iterations, iterations)
}

func TestProofAndVerify(t *testing.T) {
	tests := []struct {
		name       string
		seed       []byte
		expected   string
		iterations int
	}{
		{
			name:       "50 iterations",
			seed:       []byte{0xde, 0xad, 0xbe, 0xef},
			expected:   "0a8101001cfbcbb48b0cf0ca163c081f696211415a996a790ecf0dc7524b867900f09db5db5447bfba7eb8c269adff90d8aca5bc655b3dddfa477ee9213a8a2503c36cf35f2f8d07e4462ee610e7660faeec31fcad19401cb74aa1b28408ef183c1c111d72dea446e78d2043275911d9cab8b9820507f5134ef52d7c162e9878b805d2451281010119d48e26af61f8ef4eb95b74ddd6cb960bc0c57ead506043ddd83dfa47015f4366bcb1a9f3692d3fc272f570fe48e9ce60d613013484a1458fc3176b659f1e881239a815fd90434e2a78c879009a805ac9b29d72d864314f53a1e54b7a32af19249c57162b67f9c202ee3a323b81d4a5b4492d526accdacabeb46aa3b0bc34950a02000112020001",
			iterations: 50,
		},
		{
			name:       "100 iterations",
			seed:       []byte{0xde, 0xad, 0xbe, 0xef},
			expected:   "0a81010024ebad1ecaf920ea58f89975efe2bf97a0407a957181725c59db60c7bffbd5ead7fd519e937e0cd90f8973d2a39a18be561634bd8859ff244f993d171796a8caffe74cc56f8dfaddab8975f4c21b5bb0231b7f2d5f7d4919099ca4436104ce0ef3c711fc1c365c7303dfd0bab17a06ed40c54b9c5bd6f07a6faa181c5e2e4326128101000d5ab7b2ae4434f577dd913950ec8cecb4acac3646f7a1ede5502d63a4b5098efb8bfc3669898518eb4a53932ec884dba285a6be411e8d8df777abace1daaecc454b270c649aec55948f4daa10ab75cb2f457a1200ba9b52c06a554a13861e1ea04417101f821bfc5fb6f4138c8a8f45121e99a23d8887f576bb4dc014038a870a02000112020001",
			iterations: 100,
		},
		{
			name:       "200 iterations",
			seed:       []byte{0xde, 0xad, 0xbe, 0xef},
			expected:   "0a8101000717f503fd1e349fadac0085672df6f5b11f6a2279ef768b87aa1740d32d2c56b888fcc3ffe687f2effe719c9e7adcd424a8f8afe10ab4cda29781a04bfb41979e0b7ff755d2e52d1fd105671f176e486365eb462ceace2d19ff455e45066efe9853b6a0e1458956cc6401bc694fafec8160e8c5831fdc569bb8251899f572c1128101010223a5d74324daae5b22336f53500fba2f9a357628efd42fc407aab09291514e5dbf24d05a3387842c10f95697a9488a6477006711a11b858e4739296e479f3cd2b17a9af7f94643db4e547770c2597d9980082596dd65b26c5033dadd496c8a1ca36a54b9feea7c5f6d0a3124d1ed1593be3119fd5d625c7e92615039c4a7e50a8101005166d9a1d9978e80b1469028732fad7303f397dd4e1af5999900deb99f9a2eb824375e83a80f06a56976fae95331f05793d2b71263de186191675ea49cc319276ab0cc3a27ce4754a8e238e413bff2742d2c9637a8f10c32d3c515b53104afd74ad0060763ae37e5bc2675c344b7bb687722f99e4a8ee471633c337fcc4cde4a12810100211495c3299c094a7ab930c25499e874bb46f8d7371596f83818078e061b726f9557feaa77221d04b51d19f6ff9367090a022e651e2acce25448539341decc072ec182d402826aa3dab6588cdfedb49817594b69c9f101dc9756c1baee0db13163f2352a962068f83cab17cfc1f78462771bce0b6253672585cb42ce385467df",
			iterations: 200,
		},
		{
			name:       "300 iterations",
			seed:       []byte{0xde, 0xad, 0xbe, 0xef},
			expected:   "0a810100083ad600b0352ed17bba5b6d89fb42b6d19a6d5a4bb8f7da86e131f97df157e84ee295d88dfb81fb8d4b28d5ac79280cd03e4f990b6ac53fb1f386b05b4e04d1e7489132b10e8e327e063ce89f04b104157d19095a56c34a9166d70297876f2fedeacc6fb946efa132537dce1600ca82385708a7f985fc22c8ee44c6c689c8fc1281010102c1c3fd377fbbba18daea2b82929e26ec240d9542092de88e242e93f46a4e34c4758fde4059906196349929727c60faf03fe36c69bda82d2e1e54b524886a95e1aa2a7248cbcab90508bbee8b7c05ad4b7c23330b43a59d3a930e7e4d15db6d3449d07c13fcab42decfecd58f704a9cfbe57d45cbd11a22c91dc13499c14bd90a810100012525d26113b81d76d425d925e2cd013f34c5b24e7b5bfa797d27fd0ecc30e6fe7cd99dbf704e7a8486c316d8f389cf355fe02b87144e561c863f7fd38b56598d1bd2332ac56ac267193e1a577f9b480a5874a65b4461ca773a879acea0ac08ecafd56ecf43bdc96042b9c973f0e32385f8e3d0a3f58ee1b3823156ab3457ae128001014ab3921b8a4101a35632a991efa81230c8e76676bf704a8d2cbfedfb10ac2f39dd635bd36350cf0cf2dc7c4156b18fc0ffcb394e673b4db15f657c9110890894e419848d8efe8cc509368f7e8a2e8145b98d934c41f3e0ce69af18794caeb01d73852e9c9abf1ceee4b91f9751b4df6360e0cb92febfd6a48e53e1463bae07",
			iterations: 300,
		},
		{
			name:       "550 iterations",
			seed:       []byte{0xde, 0xad, 0xbe, 0xef},
			expected:   "0a8101001c2805cf65f702826d856e464fe5fee8d44a6643865facaae6bb1b03a0606a0c45cce2f3ad56014056bb72c5b83f7d8f429de1d12740203276c77f6d232427dec5e7b2d128c1fb55ba2db00df4a50dfa8245c8da7d47f3aba8f58e4e120216769a1594aefe338db8f3e4629e861936fad5b1cfdcea6cfd54a67614fde1a10c461281010009616a503833cf05057707105b27bea5a9cd025017382f351fe501768e8d85c7b7812056b4a41f3fe1678aa081c801d59e096ba3b90e9e2552edf3b573fd11f66d01f673ae0b489ce199bf60ea6fc7dfe316522f7985dd1acf619b78a1800eb921023700089603f94420e71eade0df8b38447f7cc32bc991ce64b295e4dfac2d0a810100574217027eb2128f8444c3173c5f83b0965377c5b9ef8fbdcc559a5233deffd33521bcfbb94adbe8027453ac8eb3668bca64fbc329655e42821db45a9ffe55eb59ab0fb5f65ad1de84afc89a842522ef14113d99b95fc386ec682f0d95ee06cd0521fefa632b34d8243a0e5bc37c704b689a5e0e53579b546ca5b59991170614128101003503a55ecc7c26a1df73e6f601f616c39cec1ac7af38970d88c32837d8ec564dd6d607245a5c55b1c0182bc6a6356bb84816a76b35df367a6b20904056f5f17bca1c9bc35cc77d56e5cb7148b5f157bbfa246cc0780d330eea636e11f869b7b2c951085062b188be3292d0ce7703bad0252619e7fc3aff79a4918e75bc62e517",
			iterations: 550,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			y, proof := GenerateVDF(test.seed, test.iterations, nil)
			got := append(y, proof...)
			require.Equal(t, test.expected, hex.EncodeToString(got))
			require.Equal(t, true, VerifyVDF(test.seed, y, proof, test.iterations))
			fmt.Println(len(bip.free))
		})
	}
}

func TestGenerateAndVerifyProofSlow(t *testing.T) {
	t.Skip()
	seed := []byte{0xde, 0xad, 0xbe, 0xef}
	for T := 5; T < 1010; T++ {
		fmt.Println(T)
		y, proof := GenerateVDF(seed, T, nil)
		require.Equal(t, true, VerifyVDF(seed, y, proof, T), "failed when T = %d", T)
	}
}

func TestRandomInput(t *testing.T) {
	for i := 0; i < 5; i++ {
		seed := make([]byte, 32)
		rand.Read(seed)
		T := 50 + 50*i
		y, proof := GenerateVDF(seed, T, nil)
		require.NotNil(t, y)
		require.NotNil(t, proof)
		require.Equal(t, true, VerifyVDF(seed, y, proof, T), "failed when T = %d", T)
	}
}

func TestInterruptGenerator(t *testing.T) {
	seed := []byte{0xde, 0xad, 0xbe, 0xef}
	stop := make(chan struct{})
	go func() {
		time.Sleep(time.Second)
		stop <- struct{}{}
	}()
	y, proof := GenerateVDF(seed, 10000, stop)
	require.Nil(t, y)
	require.Nil(t, proof)
}

func TestVDFJSON(t *testing.T) {
	expected := &VDF{
		Proof:      Hash([]byte("proof")),
		Output:     Hash([]byte("output")),
		Iterations: 999,
	}
	// convert structure to json bytes
	gotBytes, err := json.Marshal(expected)
	require.NoError(t, err)
	// convert bytes to structure
	got := new(VDF)
	// unmarshal into bytes
	require.NoError(t, json.Unmarshal(gotBytes, got))
	// compare got vs expected
	require.EqualExportedValues(t, expected, got)
}
