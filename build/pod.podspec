Pod::Spec.new do |spec|
  spec.name         = '{{.Name}}'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/ethereum/go-ethereum'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS Siotchain Client'
  spec.source       = { :git => 'https://github.com/ethereum/go-ethereum.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Siotchain.framework'

	spec.prepare_command = <<-CMD
    curl https://siotchainstore.blob.core.windows.net/builds/siotchain-ios-all-{{.Version}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv siotchain-ios-all-{{.Version}}/Siotchain.framework Frameworks
    rm -rf siotchain-ios-all-{{.Version}}
  CMD
end
