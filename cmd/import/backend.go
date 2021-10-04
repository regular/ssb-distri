package main

import (
  "io"
  "io/ioutil"
  "os"
  "fmt"
  "log"
  "strings"
  "path/filepath"
	"github.com/distr1/distri/pb"
	"google.golang.org/protobuf/encoding/prototext"
)

type BlobWriter interface {
  io.WriteCloser
  // called to indicate completeness
  // can return an opaque handle that will be passed to WriteMeta
  String() string
  Digest() (digest interface{}, err error)
}

type Repo interface {
  NewBlobWriter(pkgname string) (writer BlobWriter, err error)
  HasPackage(pkgname string) bool
  MarkAsCurrentVersion(pkgname string, meta *pb.Meta) error
  AddPackage(pkgname string, blobDigest interface{}, meta *pb.Meta) error
}

type FSRepo struct {
  targetDir string
}

func (fsr *FSRepo) blobFilePath(pkgname string) string {
  filename := fmt.Sprintf("%v.squashfs", pkgname)
  return filepath.Join(fsr.targetDir, filename) 
}

func (fsr *FSRepo) metaFilePath(pkgname string) string {
  filename := pkgname + ".meta.textproto"
  return filepath.Join(fsr.targetDir, filename) 
}

func (fsr FSRepo) NewBlobWriter(pkgname string) (writer BlobWriter, err error) {
	file, err := ioutil.TempFile(fsr.targetDir, "download-*.part")
  if err != nil {return nil, err}
  targetPath := fsr.blobFilePath(pkgname)
  return FileBlobWriter{file, targetPath}, err
}

type FileBlobWriter struct {
  *os.File
  targetPath string
}

func (fbw FileBlobWriter) Digest() (digest interface{}, err error) {
  err = nil
  digest = fbw.Name()
  return
}

func (fbw FileBlobWriter) String() string {
  return fbw.Name()
}

func (fsr FSRepo) HasPackage(pkgname string) bool {
  targetPath := fsr.blobFilePath(pkgname)
  _, err := os.Stat(targetPath)
  return err == nil 
}

func (fsr FSRepo) AddPackage(pkgname string, tmpName interface{}, meta *pb.Meta) error {
  err := os.Rename(tmpName.(string), fsr.blobFilePath(pkgname))
  if err != nil { return err }
  if meta == nil {
    log.Fatalf("meta for package %v is nil", pkgname)
  }
  metaString := prototext.Format(meta)
  bytes := []byte(metaString)
  metaFilePath := fsr.metaFilePath(pkgname)
  return ioutil.WriteFile(metaFilePath, bytes, 0666)
}

func (fsr FSRepo) MarkAsCurrentVersion(pkgname string, meta *pb.Meta) error {
  versionSuffix := "-"  + *meta.Version
  versionless := strings.TrimSuffix(pkgname, versionSuffix)
  target := pkgname + ".meta.textproto"
  link := fsr.metaFilePath(versionless)
  oldTarget, err := os.Readlink(link)
  if err == nil {
    if oldTarget == target { return nil }
    err = os.Remove(link)
    log.Fatal(err)
  }
  return os.Symlink(target, link)
}
