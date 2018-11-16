#!/usr/bin/env ruby
#
require 'optparse'
require 'fileutils'

@options = {
  :cluster => nil,
  :az => nil,
  :ca => {
    :key => 'ca.key',
    :key_bits => 4096,
    :config => 'ca.conf',
    :validity_days => 999999,
    :cert => 'ca.crt',
  },
  :certs => {
    :key_bits => 2048,
    :key => 'sidecar-injector.key',
    :csr => 'sidecar-injector.csr',
    :cert => 'sidecar-injector.crt',
    :validity_days => 999999,
    :csr_config => 'csr-prod.conf',
  }
}

def gen_new_certs(az, cluster)
  puts "Generating certs for #{az}-#{cluster}"
  workdir = "./#{az}/#{cluster}"
  # gen new ca key
  puts "Generating a new CA key..."
  if !Kernel.system(%[
        openssl \
        genrsa \
        -out #{File.join(workdir, @options[:ca][:key])} \
        #{@options[:ca][:key_bits]} \
      ])
    abort "unable to generate ca key"
  end
  # # Create and self sign the Root Certificate
  puts "Creating and signing the ca.crt (press enter when prompted for input)"
  if !Kernel.system(%[openssl req -x509 \
                      -config #{@options[:ca][:config]} \
                      -new -nodes \
                      -key #{File.join(workdir,@options[:ca][:key])} \
                      -sha256 -days #{@options[:ca][:validity_days]} \
                      -out #{File.join(workdir,@options[:ca][:cert])} \
                    ])
    abort "unable to generate and sign ca.crt"
    # <press enter a bunch>
  end
  # ### Create the certificate key
  puts "Creating the certificate key"
  if !Kernel.system(%[
        openssl genrsa \
        -out #{File.join(workdir, @options[:certs][:key])} \
        #{@options[:certs][:key_bits]} \
      ])
    abort "unable to create certificate key"
  end
  # #create signing request
  puts "Creating signing request (press enter when prompted for input)"
  if !Kernel.system(%[
        openssl req -new  \
        -key #{File.join(workdir,@options[:certs][:key])} \
        -out #{File.join(workdir,@options[:certs][:csr])} \
        -config #{@options[:certs][:csr_config]} \
      ])
    abort "unable to create CSR"
  end

  # ### Check the signing request
  # openssl req -text -noout -in sidecar-injector.csr|grep -A4 "Requested Extensions"
  #
  # ### Generate the certificate using the mydomain csr and key along with the CA Root key
  puts "Generate the cert"
  if !Kernel.system(%[
        openssl x509 -req \
        -in #{File.join(workdir,@options[:certs][:csr])} \
        -CA #{File.join(workdir, @options[:ca][:cert])} \
        -CAkey #{File.join(workdir, @options[:ca][:key])} \
        -CAcreateserial \
        -out #{File.join(workdir, @options[:certs][:cert])} \
        -days #{@options[:certs][:validity_days]} \
        -sha256 -extensions req_ext \
        -extfile #{@options[:certs][:csr_config]} \
      ])
    abort "Unable to generate and sign the cert!"
  end
end

def main
  Dir.chdir(File.dirname(__FILE__))
  if ENV['DEPLOYMENT'].nil?
      abort "You must pass DEPLOYMENT= in the environment (i.e. us-east-1 or dc01) for what DEPLOYMENT we are generating a cert for"
  end
  if ENV['CLUSTER'].nil?
      abort "You must pass CLUSTER= in the environment (i.e. PRODUCTION) for what CLUSTER we are generating a cert for"
  end
  az = ENV['DEPLOYMENT'].downcase
  cluster = ENV['CLUSTER'].upcase
  if !Dir.exist?("./#{az}/#{cluster}")
    puts "Creating ./#{az}/#{cluster}"
    FileUtils.mkdir_p("./#{az}/#{cluster}")
  end
  gen_new_certs(az, cluster)
  puts "\n\n\nAll done!\n\nHere are your certs for #{az}-#{cluster}\n\n"
  Kernel.system(%[git status])
  puts "Generated new certs for #{az}-#{cluster} for k8s-sidecar-injector"
  puts "Please commit these!"
end

main
