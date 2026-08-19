env "local" {
  src = "file://migrations"

  dev = "postgres://postgres:postgres@localhost:5432/shiyao_dev?sslmode=disable"

  migration {
    dir = "file://migrations"
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
