package dicom

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"

	"github.com/suyashkumar/dicom/pkg/dicomio"
	"github.com/suyashkumar/dicom/pkg/tag"
	"github.com/suyashkumar/dicom/pkg/uid"
)

/*
FURTHER TESTING
	- Read written values back in and verify Datsets are the same
	- With 'wild' DICOMs with high variability, read in, write out, read in, and verify
*/

func TestWrite(t *testing.T) {
	cases := []struct {
		name          string
		dataset       Dataset
		expectedError error
	}{
		{
			name: "basic types",
			dataset: Dataset{Elements: []*Element{
				mustNewElement(tag.MediaStorageSOPClassUID, []string{"1.2.840.10008.5.1.4.1.1.1.2"}),
				mustNewElement(tag.MediaStorageSOPInstanceUID, []string{"1.2.3.4.5.6.7"}),
				mustNewElement(tag.TransferSyntaxUID, []string{uid.ImplicitVRLittleEndian}),
				mustNewElement(tag.PatientName, []string{"Robin Banks"}),
				mustNewElement(tag.Rows, []int{128}),
				mustNewElement(tag.FloatingPointValue, []float64{128.10}),
			}},
			expectedError: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := ioutil.TempFile("", "write_test.dcm")
			if err != nil {
				t.Fatalf("Unexpected error when creating tempfile: %v", err)
			}
			if err = Write(file, tc.dataset); err != tc.expectedError {
				t.Errorf("Write(%v): unexpected error. got: %v, want: %v", tc.dataset, err, tc.expectedError)
			}
			file.Close()

			// Read the data back in and check for equality to the tc.dataset:
			f, err := os.Open(file.Name())
			if err != nil {
				t.Fatalf("Unexpected error opening file %s: %v", file.Name(), err)
			}
			info, err := f.Stat()
			if err != nil {
				t.Fatalf("Unexpected error state file: %s: %v", file.Name(), err)
			}

			readDS, err := Parse(f, info.Size(), nil)
			if err != nil {
				t.Errorf("Parse of written file, unexpected error: %v", err)
			}

			if diff := cmp.Diff(
				readDS,
				tc.dataset,
				cmp.AllowUnexported(allValues...),
				cmpopts.IgnoreFields(Element{}, "ValueLength"),
				cmpopts.IgnoreSliceElements(func(e *Element) bool { return e.Tag == tag.FileMetaInformationGroupLength }),
			); diff != "" {
				t.Errorf("Reading Written dataset led to unexpected diff: %s", diff)
			}
		})
	}
}

func TestEncodeElementHeader(t *testing.T) {}

func TestWriteValue(t *testing.T) {}

func TestWriteTag(t *testing.T) {}

func TestWriteVRVL(t *testing.T) {}

func TestVerifyVR(t *testing.T) {
	cases := []struct {
		name    string
		tg      tag.Tag
		inVR    string
		wantVR  string
		wantErr bool
	}{
		{
			name:    "wrong vr",
			tg:      tag.FileMetaInformationGroupLength,
			inVR:    "OB",
			wantVR:  "",
			wantErr: true,
		},
		{
			name:    "no vr",
			tg:      tag.FileMetaInformationGroupLength,
			inVR:    "",
			wantVR:  "UL",
			wantErr: false,
		},
		{
			name: "made up tag",
			tg: tag.Tag{
				Group:   0x9999,
				Element: 0x9999,
			},
			inVR:    "",
			wantVR:  "UN",
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vr, err := verifyVROrDefault(tc.tg, tc.inVR)
			if (err != nil && !tc.wantErr) || (err == nil && tc.wantErr) {
				t.Errorf("verifyVROrDefault(%v, %v), got err: %v but want err: %v", tc.tg, tc.inVR, err, tc.wantErr)
			}
			if vr != tc.wantVR {
				t.Errorf("verifyVROrDefault(%v, %v): unexpected vr. got: %v, want: %v", tc.tg, tc.inVR, vr, tc.wantVR)
			}
		})
	}
}

func TestVerifyValueType(t *testing.T) {
	cases := []struct {
		name      string
		tg        tag.Tag
		value     Value
		vr        string
		wantError bool
	}{
		{
			name:      "valid",
			tg:        tag.FileMetaInformationGroupLength,
			value:     mustNewValue([]int{128}),
			vr:        "UL",
			wantError: false,
		},
		{
			name:      "invalid vr",
			tg:        tag.FileMetaInformationGroupLength,
			value:     mustNewValue([]int{128}),
			vr:        "NA",
			wantError: true,
		},
		{
			name:      "wrong valueType",
			tg:        tag.FileMetaInformationGroupLength,
			value:     mustNewValue([]string{"str"}),
			vr:        "UL",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyValueType(tc.tg, tc.value, tc.vr)
			if (err != nil && !tc.wantError) || (err == nil && tc.wantError) {
				t.Errorf("verifyValueType(%v, %v, %v), got err: %v but want err: %v", tc.tg, tc.value, tc.vr, err, tc.wantError)
			}
		})
	}
}

func TestWriteFloats(t *testing.T) {
	// TODO: add additional cases
	cases := []struct {
		name         string
		value        Value
		vr           string
		expectedData []byte
		expectedErr  error
	}{
		{
			name:  "float64s",
			value: &floatsValue{value: []float64{20.1019, 21.212}},
			vr:    "FD",
			// TODO: improve test expectation
			expectedData: []byte{0x60, 0x76, 0x4f, 0x1e, 0x16, 0x1a, 0x34, 0x40, 0x83, 0xc0, 0xca, 0xa1, 0x45, 0x36, 0x35, 0x40},
			expectedErr:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			w := dicomio.NewWriter(&buf, binary.LittleEndian, false)
			err := writeFloats(w, tc.value, tc.vr)
			if err != tc.expectedErr {
				t.Errorf("writeFloats(%v, %s) returned unexpected err. got: %v, want: %v", tc.value, tc.vr, err, tc.expectedErr)
			}
			if diff := cmp.Diff(tc.expectedData, buf.Bytes()); diff != "" {
				t.Errorf("writeFloats(%v, %s) wrote unexpected data. diff: %s", tc.value, tc.vr, diff)
				t.Errorf("% x", buf.Bytes())
			}
		})
	}

}