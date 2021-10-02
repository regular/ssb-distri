package main

import (
  "io"
  "io/ioutil"
  "os"
  "fmt"
  "path/filepath"
	"github.com/distr1/distri/pb"
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
  AddPackage(pkgname string, blobDigest interface{}, meta *pb.Meta) error
}

type FSRepo struct {
  targetDir string
}

func (fsr *FSRepo) filepath(pkgname string) string {
  filename := fmt.Sprintf("%v.squashfs", pkgname)
  return filepath.Join(fsr.targetDir, filename) 
}

func (fsr FSRepo) NewBlobWriter(pkgname string) (writer BlobWriter, err error) {
	file, err := ioutil.TempFile(fsr.targetDir, "download-*.part")
  if err != nil {return nil, err}
  targetPath := fsr.filepath(pkgname)
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
  targetPath := fsr.filepath(pkgname)
  _, err := os.Stat(targetPath)
  return err == nil 
}

func (fsr FSRepo) AddPackage(pkgname string, tmpName interface{}, meta *pb.Meta) error {
  err := os.Rename(tmpName.(string), fsr.filepath(pkgname))
  if err != nil { return err }
  return nil
}
